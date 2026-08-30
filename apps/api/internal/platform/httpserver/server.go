// Package httpserver assembles the API transport and operational endpoints.
package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"opora.local/api/internal/audit"
	"opora.local/api/internal/auth"
	"opora.local/api/internal/config"
	"opora.local/api/internal/document"
	"opora.local/api/internal/organization"
	"opora.local/api/internal/platform/apierror"
	"opora.local/api/internal/platform/requestid"
	"opora.local/api/internal/student"
	"opora.local/api/internal/studentaccess"
	"opora.local/api/internal/user"
)

// ReadinessChecker verifies a required API dependency.
type ReadinessChecker interface {
	Ping(context.Context) error
}

// Application contains the domain HTTP handlers mounted by the modular monolith.
type Application struct {
	Auth         *auth.Handler
	Students     *student.Handler
	Documents    *document.Handler
	Users        *user.Handler
	Access       *studentaccess.Handler
	Audit        *audit.Handler
	Organization *organization.Handler
	WebOrigin    string
}

// New creates an HTTP server with bounded timeouts and safe middleware.
func New(cfg config.HTTP, logger *slog.Logger, readiness ReadinessChecker, applications ...Application) *http.Server {
	router := chi.NewRouter()
	router.Use(requestid.Middleware)
	router.Use(requestLogger(logger))
	router.Use(recoverer(logger))
	if len(applications) > 0 {
		router.Use(cors(applications[0].WebOrigin))
	}

	router.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := readiness.Ping(ctx); err != nil {
			apierror.Write(w, r, http.StatusServiceUnavailable, "service_unavailable", "Service unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	if len(applications) > 0 {
		mountApplication(router, applications[0])
	}

	return &http.Server{
		Addr:              cfg.Address,
		Handler:           router,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}

func mountApplication(router chi.Router, app Application) {
	router.Route("/api/v1", func(api chi.Router) {
		api.Use(app.Auth.OptionalSession)
		api.With(requireOrigin(app.WebOrigin)).Post("/auth/login", app.Auth.Login)
		api.With(requireOrigin(app.WebOrigin)).Post("/auth/invitations/accept", app.Users.AcceptInvitation)
		api.Group(func(protected chi.Router) {
			protected.Use(app.Auth.RequireSession)
			protected.Get("/me", app.Auth.Me)
			protected.With(requireOrigin(app.WebOrigin), app.Auth.RequireCSRF).Patch("/me", app.Users.UpdateProfile)
			protected.With(requireOrigin(app.WebOrigin), app.Auth.RequireCSRF).Post("/me/change-password", app.Users.ChangePassword)
			protected.With(requireOrigin(app.WebOrigin), app.Auth.RequireCSRF).Post("/auth/logout", app.Auth.Logout)
			protected.Get("/students", app.Students.List)
			protected.Get("/students/{studentId}", app.Students.Get)
			protected.With(requireOrigin(app.WebOrigin), app.Auth.RequireCSRF).Post("/students", app.Students.Create)
			protected.With(requireOrigin(app.WebOrigin), app.Auth.RequireCSRF).Patch("/students/{studentId}", app.Students.Update)
			protected.Get("/roles", app.Users.Roles)
			protected.Get("/users", app.Users.List)
			protected.With(requireOrigin(app.WebOrigin), app.Auth.RequireCSRF).Post("/users", app.Users.Create)
			protected.With(requireOrigin(app.WebOrigin), app.Auth.RequireCSRF).Post("/users/{userId}/invitation", app.Users.ResendInvitation)
			protected.With(requireOrigin(app.WebOrigin), app.Auth.RequireCSRF).Patch("/organization", app.Organization.Update)
			protected.Get("/students/{studentId}/access", app.Access.List)
			protected.With(requireOrigin(app.WebOrigin), app.Auth.RequireCSRF).Post("/students/{studentId}/access", app.Access.Set)
			protected.With(requireOrigin(app.WebOrigin), app.Auth.RequireCSRF).Patch("/students/{studentId}/access", app.Access.Set)
			protected.With(requireOrigin(app.WebOrigin), app.Auth.RequireCSRF).Delete("/students/{studentId}/access/{userId}", app.Access.Delete)
			protected.Get("/audit", app.Audit.List)
			protected.Get("/students/{studentId}/documents", app.Documents.List)
			protected.With(requireOrigin(app.WebOrigin), app.Auth.RequireCSRF).Post("/students/{studentId}/documents", app.Documents.Upload)
			protected.Get("/documents/{documentId}", app.Documents.Get)
			protected.Get("/documents/{documentId}/versions", app.Documents.Versions)
			protected.Get("/documents/{documentId}/download", app.Documents.Download)
			protected.Get("/documents/{documentId}/preview", app.Documents.Preview)
			protected.Get("/documents/{documentId}/versions/{versionId}/download", app.Documents.DownloadVersion)
			protected.Get("/documents/{documentId}/editor", app.Documents.Editor)
		})
		api.Get("/internal/onlyoffice/files/{documentId}/{versionId}", app.Documents.InternalFile)
		api.Post("/internal/onlyoffice/callback/{documentId}", app.Documents.Callback)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			response := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(response, r)
			logger.InfoContext(r.Context(), "http request completed",
				"request_id", requestid.FromContext(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", response.Status(),
				"duration_ms", time.Since(started).Milliseconds(),
			)
		})
	}
}

func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			defer func(ctx context.Context) {
				if recover() != nil {
					logger.ErrorContext(ctx, "panic recovered",
						"request_id", requestid.FromContext(ctx),
						"stack", string(debug.Stack()),
					)
					apierror.Write(w, r, http.StatusInternalServerError, "internal_error", "Internal server error")
				}
			}(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

func cors(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == allowedOrigin {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Add("Vary", "Origin")
			}
			if r.Method == http.MethodOptions {
				if origin != allowedOrigin {
					apierror.Write(w, r, http.StatusForbidden, "origin_denied", "Origin denied")
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requireOrigin(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !auth.OriginAllowed(r, allowedOrigin) {
				apierror.Write(w, r, http.StatusForbidden, "origin_denied", "Origin denied")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
