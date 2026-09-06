package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ochidoma/platform/internal/models"
)

type AnnouncementRepo struct {
	pool *pgxpool.Pool
}

func NewAnnouncementRepo(pool *pgxpool.Pool) *AnnouncementRepo {
	return &AnnouncementRepo{pool: pool}
}

func (r *AnnouncementRepo) Pool() *pgxpool.Pool {
	return r.pool
}

const announcementColumns = `id, reference_number, title, category, content, status,
	is_official, is_demo_content, published_at, created_at`

func (r *AnnouncementRepo) ListPublished(ctx context.Context) ([]models.Announcement, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+announcementColumns+` FROM announcements
		WHERE status = 'published' ORDER BY published_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Announcement{}
	for rows.Next() {
		a, err := scanAnnouncement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *AnnouncementRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Announcement, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+announcementColumns+` FROM announcements
		WHERE id = $1 AND status = 'published'`, id)
	a, err := scanAnnouncementRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// --- admin methods: create -> submit -> approve -> publish -> archive ---

func (r *AnnouncementRepo) AdminList(ctx context.Context, status string) ([]models.Announcement, error) {
	query := `SELECT ` + announcementColumns + ` FROM announcements`
	var args []any
	if status != "" {
		query += ` WHERE status = $1`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Announcement{}
	for rows.Next() {
		a, err := scanAnnouncement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *AnnouncementRepo) AdminGetByID(ctx context.Context, id uuid.UUID) (*models.Announcement, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+announcementColumns+` FROM announcements WHERE id = $1`, id)
	a, err := scanAnnouncementRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

type AnnouncementInput struct {
	Title    string
	Category *string
	Content  string
}

func (r *AnnouncementRepo) AdminCreate(ctx context.Context, in AnnouncementInput, authorID uuid.UUID) (*models.Announcement, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO announcements (title, category, content, author_id, status)
		VALUES ($1, $2, $3, $4, 'draft')
		RETURNING id`, in.Title, in.Category, in.Content, authorID).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.AdminGetByID(ctx, id)
}

// AdminUpdate edits a draft's title/content. Blocked by the DB trigger if the
// row is already published — callers should check status before calling this
// (or just rely on the trigger's error and surface it as 409 Conflict).
func (r *AnnouncementRepo) AdminUpdate(ctx context.Context, id uuid.UUID, in AnnouncementInput) (*models.Announcement, error) {
	_, err := r.pool.Exec(ctx, `
		UPDATE announcements SET title=$1, category=$2, content=$3, updated_at=now() WHERE id=$4`,
		in.Title, in.Category, in.Content, id)
	if err != nil {
		return nil, err
	}
	return r.AdminGetByID(ctx, id)
}

func (r *AnnouncementRepo) AdminSubmit(ctx context.Context, id uuid.UUID) (*models.Announcement, error) {
	_, err := r.pool.Exec(ctx, `UPDATE announcements SET status='in_review', updated_at=now() WHERE id=$1 AND status='draft'`, id)
	if err != nil {
		return nil, err
	}
	return r.AdminGetByID(ctx, id)
}

func (r *AnnouncementRepo) AdminApprove(ctx context.Context, id uuid.UUID) (*models.Announcement, error) {
	_, err := r.pool.Exec(ctx, `UPDATE announcements SET status='approved', updated_at=now() WHERE id=$1 AND status='in_review'`, id)
	if err != nil {
		return nil, err
	}
	return r.AdminGetByID(ctx, id)
}

// AdminPublish assigns a reference number, stamps publish metadata, and
// moves status to published. Requires the row to currently be 'approved'.
func (r *AnnouncementRepo) AdminPublish(ctx context.Context, id uuid.UUID, publishedBy uuid.UUID) (*models.Announcement, error) {
	year := time.Now().Year()
	ref := fmt.Sprintf("PALACE-ANNOUNCEMENT-%d-%s", year, id.String()[:8])

	tag, err := r.pool.Exec(ctx, `
		UPDATE announcements
		SET status='published', reference_number=$1, published_by=$2, published_at=now(), updated_at=now()
		WHERE id=$3 AND status='approved'`, ref, publishedBy, id)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrInvalidTransition
	}
	return r.AdminGetByID(ctx, id)
}

func (r *AnnouncementRepo) AdminArchive(ctx context.Context, id uuid.UUID) (*models.Announcement, error) {
	_, err := r.pool.Exec(ctx, `UPDATE announcements SET status='archived', updated_at=now() WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	return r.AdminGetByID(ctx, id)
}

var ErrInvalidTransition = fmt.Errorf("content is not in the required status for this action")

func scanAnnouncement(rows pgx.Rows) (models.Announcement, error) {
	return scanAnnouncementRow(rows)
}

func scanAnnouncementRow(row rowScanner) (models.Announcement, error) {
	var a models.Announcement
	err := row.Scan(&a.ID, &a.ReferenceNumber, &a.Title, &a.Category, &a.Content,
		&a.Status, &a.IsOfficial, &a.IsDemo, &a.PublishedAt, &a.CreatedAt)
	return a, err
}
