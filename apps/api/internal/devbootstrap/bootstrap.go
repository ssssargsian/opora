// Package devbootstrap creates an explicitly configured local development account.
package devbootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"opora.local/api/internal/auth"
	"opora.local/api/internal/config"
)

func Run(ctx context.Context, pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) error {
	if cfg.Environment == "production" || cfg.Dev.AdminPassword == "" {
		return nil
	}
	email := strings.ToLower(strings.TrimSpace(cfg.Dev.AdminEmail))
	if email == "" {
		return errors.New("DEV_ADMIN_EMAIL is empty")
	}
	passwordHash, err := auth.HashPassword(cfg.Dev.AdminPassword)
	if err != nil {
		return fmt.Errorf("hash development password: %w", err)
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID, organizationID, roleID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT u.id,m.organization_id,m.role_id FROM users u
		JOIN memberships m ON m.user_id=u.id WHERE u.email=$1 LIMIT 1`, email).Scan(&userID, &organizationID, &roleID)
	if errors.Is(err, pgx.ErrNoRows) {
		organizationID, err = uuid.NewV7()
		if err != nil {
			return err
		}
		userID, err = uuid.NewV7()
		if err != nil {
			return err
		}
		roleID, err = uuid.NewV7()
		if err != nil {
			return err
		}
		membershipID, newErr := uuid.NewV7()
		if newErr != nil {
			return newErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO organizations(id,name) VALUES($1,$2)`, organizationID, cfg.Dev.Organization); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO users(id,email,password_hash,display_name,last_name,first_name) VALUES($1,$2,$3,'Администратор Опоры','Администратор','Опоры')`, userID, email, passwordHash); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO roles(id,organization_id,role_key,name,is_system) VALUES($1,$2,'organization_admin','Администратор организации',true)`, roleID, organizationID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO memberships(id,organization_id,user_id,role_id,is_active,all_students) VALUES($1,$2,$3,$4,true,true)`, membershipID, organizationID, userID, roleID); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if _, err = tx.Exec(ctx, `UPDATE users SET password_hash=$1,is_active=true,updated_at=now() WHERE id=$2`, passwordHash, userID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE memberships SET is_active=true,all_students=true,updated_at=now() WHERE organization_id=$1 AND user_id=$2`, organizationID, userID); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_code)
		SELECT $1,code FROM permissions ON CONFLICT DO NOTHING`, roleID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO roles(id,organization_id,role_key,name,is_system,default_all_students)
		SELECT md5(($1::uuid)::text || ':' || v.role_key)::uuid,$1::uuid,v.role_key,v.name,true,false
		FROM (VALUES ('psychologist','Психолог'),('specialist','Специалист'),('viewer','Просмотр')) AS v(role_key,name)
		ON CONFLICT (organization_id,role_key) DO UPDATE SET name=excluded.name,is_system=true`, organizationID); err != nil {
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
		) AS v(role_key,permission_code) ON v.role_key=r.role_key
		WHERE r.organization_id=$1 ON CONFLICT DO NOTHING`, organizationID); err != nil {
		return err
	}
	var studentCount int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM students WHERE organization_id=$1`, organizationID).Scan(&studentCount); err != nil {
		return err
	}
	if studentCount == 0 {
		studentID, newErr := uuid.NewV7()
		if newErr != nil {
			return newErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO students(id,organization_id,last_name,first_name,middle_name,birth_date,class_name)
			VALUES($1,$2,'Иванов','Иван','Иванович','2013-03-14','7А')`, studentID, organizationID); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO student_access_grants(organization_id,student_id,user_id,grant_code)
		SELECT s.organization_id,s.id,$2,g.grant_code
		FROM students s
		CROSS JOIN (VALUES ('view'),('upload'),('edit'),('download')) AS g(grant_code)
		WHERE s.organization_id=$1
		ON CONFLICT DO NOTHING`, organizationID, userID); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	logger.Info("development account ready", "service", "api", "email", email, "organization_id", organizationID)
	return nil
}
