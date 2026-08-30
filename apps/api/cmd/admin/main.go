// Command admin performs production-safe, one-off administrative provisioning.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"net/mail"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"opora.local/api/internal/auth"
	"opora.local/api/internal/platform/database"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "create" {
		fatal("usage: opora-admin create --email EMAIL --name NAME --organization ORGANIZATION")
	}
	flags := flag.NewFlagSet("create", flag.ExitOnError)
	email := flags.String("email", "", "administrator email")
	name := flags.String("name", "Администратор организации", "administrator display name")
	organization := flags.String("organization", "Опора", "organization name")
	if err := flags.Parse(os.Args[2:]); err != nil {
		fatal(err.Error())
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fatal("DATABASE_URL is required")
	}
	normalizedEmail, err := normalizeEmail(*email)
	if err != nil {
		fatal(err.Error())
	}
	displayName := strings.TrimSpace(*name)
	organizationName := strings.TrimSpace(*organization)
	if displayName == "" || len([]rune(displayName)) > 255 {
		fatal("name must contain 1 to 255 characters")
	}
	if organizationName == "" || len([]rune(organizationName)) > 255 {
		fatal("organization must contain 1 to 255 characters")
	}

	password, err := generatePassword()
	if err != nil {
		fatal("could not generate a password")
	}
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		fatal("could not hash the password")
	}

	ctx := context.Background()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		fatal("database is unavailable")
	}
	defer pool.Close()
	if err := createFirstAdmin(ctx, pool, normalizedEmail, displayName, organizationName, passwordHash); err != nil {
		if errors.Is(err, errUserExists) {
			fatal("a user with this email already exists; no password was changed")
		}
		fatal("first administrator could not be created")
	}

	// The generated password is intentionally emitted only after the transaction commits.
	fmt.Printf("First administrator created.\nEmail: %s\nInitial password (shown once): %s\n", normalizedEmail, password)
}

type databasePool interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

var errUserExists = errors.New("user already exists")

func createFirstAdmin(ctx context.Context, pool databasePool, email, name, organization, passwordHash string) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email=$1)`, email).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return errUserExists
	}

	organizationID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	userID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	adminRoleID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	membershipID, err := uuid.NewV7()
	if err != nil {
		return err
	}

	if _, err = tx.Exec(ctx, `INSERT INTO organizations(id,name) VALUES($1,$2)`, organizationID, organization); err != nil {
		return err
	}
	lastName, firstName := splitDisplayName(name)
	if _, err = tx.Exec(ctx, `INSERT INTO users(id,email,password_hash,display_name,last_name,first_name) VALUES($1,$2,$3,$4,$5,$6)`,
		userID, email, passwordHash, name, lastName, firstName); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO roles(id,organization_id,role_key,name,is_system,default_all_students)
		VALUES($1,$2,'organization_admin','Администратор организации',true,true)`, adminRoleID, organizationID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO memberships(id,organization_id,user_id,role_id,is_active,all_students)
		VALUES($1,$2,$3,$4,true,true)`, membershipID, organizationID, userID, adminRoleID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_code)
		SELECT $1,code FROM permissions`, adminRoleID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO roles(id,organization_id,role_key,name,is_system,default_all_students)
		SELECT md5(($1::uuid)::text || ':' || v.role_key)::uuid,$1::uuid,v.role_key,v.name,true,false
		FROM (VALUES ('psychologist','Психолог'),('specialist','Специалист'),('viewer','Просмотр')) AS v(role_key,name)`, organizationID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_code)
		SELECT r.id,v.permission_code FROM roles r JOIN (VALUES
			('psychologist','students.list'),('psychologist','students.view'),('psychologist','students.create'),
			('psychologist','students.update'),('psychologist','documents.list'),('psychologist','documents.view'),
			('psychologist','documents.download'),('psychologist','documents.upload'),('psychologist','documents.edit'),
			('specialist','students.list'),('specialist','students.view'),('specialist','students.create'),
			('specialist','documents.list'),('specialist','documents.view'),('specialist','documents.download'),
			('specialist','documents.upload'),('specialist','documents.edit'),('viewer','students.list'),
			('viewer','students.view'),('viewer','documents.list'),('viewer','documents.view'),('viewer','documents.download')
		) AS v(role_key,permission_code) ON v.role_key=r.role_key WHERE r.organization_id=$1`, organizationID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func splitDisplayName(value string) (string, string) {
	parts := strings.Fields(value)
	if len(parts) < 2 {
		return value, "Пользователь"
	}
	return parts[0], strings.Join(parts[1:], " ")
}

func normalizeEmail(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	parsed, err := mail.ParseAddress(normalized)
	if err != nil || parsed.Address != normalized || len(normalized) > 320 {
		return "", errors.New("email is invalid")
	}
	return normalized, nil
}

func generatePassword() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "Op-" + base64.RawURLEncoding.EncodeToString(bytes), nil
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
