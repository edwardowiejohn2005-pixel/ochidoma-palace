// Package audit records every admin mutation. Handlers call Log within the
// same DB transaction as the mutation itself, so if the audit write fails,
// the mutation rolls back too — you cannot silently change content without
// leaving a trail.
package audit

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type Action string

const (
	ActionAdminLogin          Action = "ADMIN_LOGIN"
	ActionAdminLogout         Action = "ADMIN_LOGOUT"
	ActionArticleCreated      Action = "ADMIN_CREATED_ARTICLE"
	ActionArticleUpdated      Action = "ADMIN_UPDATED_DRAFT"
	ActionArticlePublished    Action = "ARTICLE_PUBLISHED"
	ActionEventCreated        Action = "ADMIN_CREATED_EVENT"
	ActionAnnouncementCreated Action = "ANNOUNCEMENT_CREATED"
	ActionAnnouncementPublish Action = "ANNOUNCEMENT_PUBLISHED"
	ActionDecreeCreated       Action = "DECREE_CREATED"
	ActionDecreeSubmitted     Action = "DECREE_SUBMITTED"
	ActionDecreeApproved      Action = "DECREE_APPROVED"
	ActionDecreePublished     Action = "DECREE_PUBLISHED"
	ActionDecreeArchived      Action = "DECREE_ARCHIVED"
	ActionDecreeCorrected     Action = "DECREE_CORRECTED"
	ActionMediaUploaded       Action = "MEDIA_UPLOADED"
)

type Entry struct {
	UserID        uuid.UUID
	Action        Action
	EntityType    string
	EntityID      uuid.UUID
	PreviousState any // marshaled to JSONB; nil for creates
	NewState      any
	IPAddress     string
}

// Log inserts an audit row using the given tx (or pool) so callers can
// compose it inside their own transaction. Pass any pgx.Tx or *pgxpool.Pool
// that satisfies the Querier interface below.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func Log(ctx context.Context, q Querier, e Entry) error {
	prevJSON, err := marshalOrNil(e.PreviousState)
	if err != nil {
		return err
	}
	newJSON, err := marshalOrNil(e.NewState)
	if err != nil {
		return err
	}

	_, err = q.Exec(ctx, `
		INSERT INTO audit_logs (user_id, action, entity_type, entity_id, previous_state, new_state, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.UserID, string(e.Action), e.EntityType, e.EntityID, prevJSON, newJSON, e.IPAddress,
	)
	return err
}

func marshalOrNil(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}
