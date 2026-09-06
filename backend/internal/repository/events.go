package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ochidoma/platform/internal/models"
)

type EventRepo struct {
	pool *pgxpool.Pool
}

func NewEventRepo(pool *pgxpool.Pool) *EventRepo {
	return &EventRepo{pool: pool}
}

func (r *EventRepo) Pool() *pgxpool.Pool {
	return r.pool
}

const eventColumns = `id, name, slug, description, starts_at, ends_at, location,
	organizer, category, status, is_demo_content, created_at`

// List returns events, optionally filtered by status (upcoming|ongoing|completed).
// Pass "" to get all of them.
func (r *EventRepo) List(ctx context.Context, status string) ([]models.Event, error) {
	query := `SELECT ` + eventColumns + ` FROM events`
	var args []any
	if status != "" {
		query += ` WHERE status = $1`
		args = append(args, status)
	}
	query += ` ORDER BY starts_at ASC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Event{}
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *EventRepo) GetBySlug(ctx context.Context, slug string) (*models.Event, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+eventColumns+` FROM events WHERE slug = $1`, slug)
	e, err := scanEventRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

// --- admin methods ---

const adminEventColumns = `id, name, slug, description, starts_at, ends_at, location,
	organizer, category, status, is_demo_content, created_by, created_at`

func (r *EventRepo) AdminList(ctx context.Context) ([]models.Event, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+adminEventColumns+` FROM events ORDER BY starts_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Event{}
	for rows.Next() {
		var e models.Event
		if err := rows.Scan(&e.ID, &e.Name, &e.Slug, &e.Description, &e.StartsAt, &e.EndsAt,
			&e.Location, &e.Organizer, &e.Category, &e.Status, &e.IsDemo, &e.CreatedBy, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *EventRepo) AdminGetByID(ctx context.Context, id uuid.UUID) (*models.Event, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+adminEventColumns+` FROM events WHERE id = $1`, id)
	var e models.Event
	err := row.Scan(&e.ID, &e.Name, &e.Slug, &e.Description, &e.StartsAt, &e.EndsAt,
		&e.Location, &e.Organizer, &e.Category, &e.Status, &e.IsDemo, &e.CreatedBy, &e.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

type EventInput struct {
	Name        string
	Slug        string
	Description *string
	StartsAt    time.Time
	EndsAt      *time.Time
	Location    *string
	Organizer   *string
	Category    *string
}

func (r *EventRepo) AdminCreate(ctx context.Context, in EventInput, createdBy uuid.UUID) (*models.Event, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO events (name, slug, description, starts_at, ends_at, location, organizer, category, created_by, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'upcoming')
		RETURNING id`,
		in.Name, in.Slug, in.Description, in.StartsAt, in.EndsAt, in.Location, in.Organizer, in.Category, createdBy).
		Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.AdminGetByID(ctx, id)
}

func (r *EventRepo) AdminUpdate(ctx context.Context, id uuid.UUID, in EventInput) (*models.Event, error) {
	_, err := r.pool.Exec(ctx, `
		UPDATE events SET name=$1, slug=$2, description=$3, starts_at=$4, ends_at=$5,
			location=$6, organizer=$7, category=$8
		WHERE id=$9`,
		in.Name, in.Slug, in.Description, in.StartsAt, in.EndsAt, in.Location, in.Organizer, in.Category, id)
	if err != nil {
		return nil, err
	}
	return r.AdminGetByID(ctx, id)
}

// AdminSetStatus transitions status to upcoming|ongoing|completed (not the
// publication_status enum — events use their own simpler lifecycle).
func (r *EventRepo) AdminSetStatus(ctx context.Context, id uuid.UUID, status string) (*models.Event, error) {
	_, err := r.pool.Exec(ctx, `UPDATE events SET status=$1 WHERE id=$2`, status, id)
	if err != nil {
		return nil, err
	}
	return r.AdminGetByID(ctx, id)
}

func (r *EventRepo) AdminDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM events WHERE id = $1`, id)
	return err
}

func scanEvent(rows pgx.Rows) (models.Event, error) {
	return scanEventRow(rows)
}

func scanEventRow(row rowScanner) (models.Event, error) {
	var e models.Event
	err := row.Scan(&e.ID, &e.Name, &e.Slug, &e.Description, &e.StartsAt, &e.EndsAt,
		&e.Location, &e.Organizer, &e.Category, &e.Status, &e.IsDemo, &e.CreatedAt)
	return e, err
}
