// Command api runs the Opora HTTP API process.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"opora.local/api/internal/audit"
	"opora.local/api/internal/auth"
	"opora.local/api/internal/config"
	"opora.local/api/internal/devbootstrap"
	"opora.local/api/internal/document"
	"opora.local/api/internal/organization"
	"opora.local/api/internal/platform/database"
	"opora.local/api/internal/platform/httpserver"
	"opora.local/api/internal/student"
	"opora.local/api/internal/studentaccess"
	"opora.local/api/internal/user"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration is invalid", "error", err)
		os.Exit(1)
	}
	if err := cfg.ValidateDocumentIntegrations(); err != nil {
		logger.Error("document integration configuration is invalid", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database initialization failed")
		os.Exit(1)
	}
	defer pool.Close()
	if err := devbootstrap.Run(ctx, pool, cfg, logger); err != nil {
		logger.Error("development bootstrap failed", "error", err)
		os.Exit(1)
	}

	authRepository := auth.NewRepository(pool)
	authService := auth.NewService(authRepository, cfg.Auth.SessionTTL)
	authHandler := auth.NewHandler(authService, cfg.Auth)
	studentRepository := student.NewRepository(pool)
	studentService := student.NewService(studentRepository)
	studentHandler := student.NewHandler(studentService)
	userRepository := user.NewRepository(pool)
	invitationMailer, err := user.NewSMTPInvitationMailer(user.SMTPSettings{
		Host: cfg.SMTP.Host, Port: cfg.SMTP.Port, Username: cfg.SMTP.Username, Password: cfg.SMTP.Password,
		FromEmail: cfg.SMTP.FromEmail, FromName: cfg.SMTP.FromName, TLSMode: cfg.SMTP.TLSMode, AppPublicURL: cfg.SMTP.AppPublicURL,
	})
	if err != nil {
		logger.Error("SMTP invitation configuration is invalid")
		os.Exit(1)
	}
	userHandler := user.NewHandler(user.NewService(userRepository, user.WithInvitationMailer(invitationMailer)))
	organizationHandler := organization.NewHandler(organization.NewService(organization.NewRepository(pool)))
	accessRepository := studentaccess.NewRepository(pool)
	accessHandler := studentaccess.NewHandler(studentaccess.NewService(accessRepository, studentRepository))
	auditHandler := audit.NewHandler(audit.NewRepository(pool))
	storage, err := document.NewS3Storage(cfg.Storage)
	if err != nil {
		logger.Error("object storage initialization failed")
		os.Exit(1)
	}
	documentRepository := document.NewRepository(pool)
	documentService := document.NewService(documentRepository, storage, document.NewClamAVScanner(cfg.ClamAV.Address), studentRepository, cfg.Upload.MaxBytes)
	onlyOfficeService := document.NewOnlyOfficeService(documentService, authRepository, cfg.OnlyOffice, cfg.Upload.MaxBytes)
	documentHandler := document.NewHandler(documentService, onlyOfficeService, cfg.Upload.MaxBytes, logger)
	server := httpserver.New(cfg.HTTP, logger, pool, httpserver.Application{
		Auth: authHandler, Students: studentHandler, Documents: documentHandler, Users: userHandler,
		Access: accessHandler, Audit: auditHandler, Organization: organizationHandler, WebOrigin: cfg.Auth.AllowedOrigin,
	})
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("server_start",
			"service", "api",
			"event", "server_start",
			"address", cfg.HTTP.Address,
			"environment", cfg.Environment,
		)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err = <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("http server stopped")
}
