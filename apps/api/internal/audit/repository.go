// Package audit owns append-only application audit events.
package audit

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Event struct {
	OrganizationID       uuid.UUID
	ActorUserID          *uuid.UUID
	Action, ResourceType string
	ResourceID           *uuid.UUID
	IPAddress, UserAgent string
	Metadata             map[string]any
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

type Entry struct {
	ID           uuid.UUID      `json:"id"`
	ActorName    string         `json:"actorName"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resourceType"`
	ResourceID   *uuid.UUID     `json:"resourceId"`
	CreatedAt    time.Time      `json:"createdAt"`
	Metadata     map[string]any `json:"metadata"`
}

func (r *Repository) List(ctx context.Context, organizationID uuid.UUID, limit int) ([]Entry, error) {
	rows, err := r.pool.Query(ctx, `SELECT e.id,COALESCE(u.display_name,'Система'),e.action,e.resource_type,
		e.resource_id,e.created_at,e.metadata
		FROM audit_events e LEFT JOIN users u ON u.id=e.actor_user_id
		WHERE e.organization_id=$1 ORDER BY e.created_at DESC,e.id DESC LIMIT $2`, organizationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Entry, 0)
	for rows.Next() {
		var entry Entry
		var metadata []byte
		if err := rows.Scan(&entry.ID, &entry.ActorName, &entry.Action, &entry.ResourceType,
			&entry.ResourceID, &entry.CreatedAt, &metadata); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(metadata, &entry.Metadata); err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}

func (r *Repository) Append(ctx context.Context, event Event) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return err
	}
	var ip any
	if parsed := net.ParseIP(event.IPAddress); parsed != nil {
		ip = parsed.String()
	}
	ua := strings.TrimSpace(event.UserAgent)
	if len(ua) > 512 {
		ua = ua[:512]
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO audit_events
		(id,organization_id,actor_user_id,action,resource_type,resource_id,ip_address,user_agent,metadata)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, event.OrganizationID, event.ActorUserID, event.Action, event.ResourceType, event.ResourceID, ip, nullString(ua), metadata)
	return err
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
