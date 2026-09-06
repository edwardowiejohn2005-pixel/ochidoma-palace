package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ochidoma/platform/internal/models"
)

type DecreeRepo struct {
	pool *pgxpool.Pool
}

func NewDecreeRepo(pool *pgxpool.Pool) *DecreeRepo {
	return &DecreeRepo{pool: pool}
}

func (r *DecreeRepo) Pool() *pgxpool.Pool {
	return r.pool
}

// ListPublished returns the latest published version of every non-archived
// decree — the only view the public API ever exposes.
func (r *DecreeRepo) ListPublished(ctx context.Context) ([]models.DecreeVersion, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT dv.id, dv.decree_id, dv.version_number, dv.title, dv.full_text,
		       dv.issuing_authority, dv.date_issued, dv.effective_date,
		       dv.integrity_hash, dv.status, dv.is_correction,
		       dv.supersedes_version_id, dv.published_at, dv.created_at
		FROM decree_versions dv
		JOIN decrees d ON d.id = dv.decree_id
		WHERE dv.status = 'published'
		  AND dv.version_number = (
		      SELECT MAX(version_number) FROM decree_versions
		      WHERE decree_id = dv.decree_id AND status = 'published'
		  )
		ORDER BY dv.published_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.DecreeVersion{}
	for rows.Next() {
		var v models.DecreeVersion
		if err := rows.Scan(&v.ID, &v.DecreeID, &v.VersionNumber, &v.Title, &v.FullText,
			&v.IssuingAuthority, &v.DateIssued, &v.EffectiveDate, &v.IntegrityHash,
			&v.Status, &v.IsCorrection, &v.SupersedesVersionID, &v.PublishedAt, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetByNumber fetches a decree and its published version by public decree_number.
func (r *DecreeRepo) GetByNumber(ctx context.Context, decreeNumber string) (*models.DecreeVersion, error) {
	var v models.DecreeVersion
	err := r.pool.QueryRow(ctx, `
		SELECT dv.id, dv.decree_id, dv.version_number, dv.title, dv.full_text,
		       dv.issuing_authority, dv.date_issued, dv.effective_date,
		       dv.integrity_hash, dv.status, dv.is_correction,
		       dv.supersedes_version_id, dv.published_at, dv.created_at
		FROM decree_versions dv
		JOIN decrees d ON d.id = dv.decree_id
		WHERE d.decree_number = $1 AND dv.status = 'published'
		ORDER BY dv.version_number DESC
		LIMIT 1`, decreeNumber).
		Scan(&v.ID, &v.DecreeID, &v.VersionNumber, &v.Title, &v.FullText,
			&v.IssuingAuthority, &v.DateIssued, &v.EffectiveDate, &v.IntegrityHash,
			&v.Status, &v.IsCorrection, &v.SupersedesVersionID, &v.PublishedAt, &v.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// --- admin methods: create -> submit -> approve -> publish, plus corrections ---

const decreeVersionColumns = `dv.id, dv.decree_id, dv.version_number, dv.title, dv.full_text,
	dv.issuing_authority, dv.date_issued, dv.effective_date, dv.integrity_hash, dv.status,
	dv.is_correction, dv.supersedes_version_id, dv.published_at, dv.created_at`

func scanDecreeVersionRow(row rowScanner) (models.DecreeVersion, error) {
	var v models.DecreeVersion
	err := row.Scan(&v.ID, &v.DecreeID, &v.VersionNumber, &v.Title, &v.FullText,
		&v.IssuingAuthority, &v.DateIssued, &v.EffectiveDate, &v.IntegrityHash,
		&v.Status, &v.IsCorrection, &v.SupersedesVersionID, &v.PublishedAt, &v.CreatedAt)
	return v, err
}

// AdminListAll returns every decree version, most recent first — the admin
// portal's view of drafts, in-review, approved, published, and archived alike.
func (r *DecreeRepo) AdminListAll(ctx context.Context) ([]models.DecreeVersion, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+decreeVersionColumns+`
		FROM decree_versions dv
		ORDER BY dv.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.DecreeVersion{}
	for rows.Next() {
		v, err := scanDecreeVersionRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *DecreeRepo) AdminGetVersion(ctx context.Context, versionID uuid.UUID) (*models.DecreeVersion, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+decreeVersionColumns+` FROM decree_versions dv WHERE dv.id = $1`, versionID)
	v, err := scanDecreeVersionRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &v, nil
}

type DecreeInput struct {
	DecreeNumber     string
	Title            string
	FullText         string
	IssuingAuthority string
}

// AdminCreate creates a new decree plus its first draft version (1.0).
func (r *DecreeRepo) AdminCreate(ctx context.Context, in DecreeInput, createdBy uuid.UUID) (*models.DecreeVersion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op if committed

	var decreeID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO decrees (decree_number, current_status) VALUES ($1, 'draft') RETURNING id`,
		in.DecreeNumber).Scan(&decreeID); err != nil {
		return nil, err
	}

	hash := contentHash(in.Title, in.FullText, in.IssuingAuthority, "1")
	var versionID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO decree_versions (decree_id, version_number, title, full_text,
			issuing_authority, integrity_hash, status, created_by)
		VALUES ($1, 1.0, $2, $3, $4, $5, 'draft', $6)
		RETURNING id`,
		decreeID, in.Title, in.FullText, in.IssuingAuthority, hash, createdBy).Scan(&versionID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.AdminGetVersion(ctx, versionID)
}

// AdminSubmit moves a draft version to in_review.
func (r *DecreeRepo) AdminSubmit(ctx context.Context, versionID uuid.UUID) (*models.DecreeVersion, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE decree_versions SET status='in_review' WHERE id=$1 AND status='draft'`, versionID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrInvalidTransition
	}
	_, _ = r.pool.Exec(ctx, `UPDATE decrees SET current_status='in_review' WHERE id = (SELECT decree_id FROM decree_versions WHERE id=$1)`, versionID)
	return r.AdminGetVersion(ctx, versionID)
}

// AdminApprove moves an in_review version to approved. Restricted to
// palace_publisher/super_admin at the handler layer.
func (r *DecreeRepo) AdminApprove(ctx context.Context, versionID uuid.UUID, approvedBy uuid.UUID) (*models.DecreeVersion, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE decree_versions SET status='approved', approved_by=$1, approved_at=now()
		WHERE id=$2 AND status='in_review'`, approvedBy, versionID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrInvalidTransition
	}
	_, _ = r.pool.Exec(ctx, `UPDATE decrees SET current_status='approved' WHERE id = (SELECT decree_id FROM decree_versions WHERE id=$1)`, versionID)
	return r.AdminGetVersion(ctx, versionID)
}

// AdminPublish moves an approved version to published: recomputes the
// integrity hash over the final content, stamps publish metadata, and
// updates the parent decree's current_status. From this point on, the DB
// trigger prevents any further edits to this version's content — only a
// correction (new version) can change it.
func (r *DecreeRepo) AdminPublish(ctx context.Context, versionID uuid.UUID, publishedBy uuid.UUID) (*models.DecreeVersion, error) {
	v, err := r.AdminGetVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}
	if v == nil || v.Status != "approved" {
		return nil, ErrInvalidTransition
	}

	hash := contentHash(v.Title, v.FullText, v.IssuingAuthority, fmt.Sprintf("%.1f", v.VersionNumber))

	tag, err := r.pool.Exec(ctx, `
		UPDATE decree_versions
		SET status='published', integrity_hash=$1, published_by=$2, published_at=now()
		WHERE id=$3 AND status='approved'`, hash, publishedBy, versionID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrInvalidTransition
	}
	_, _ = r.pool.Exec(ctx, `UPDATE decrees SET current_status='published' WHERE id = (SELECT decree_id FROM decree_versions WHERE id=$1)`, versionID)
	return r.AdminGetVersion(ctx, versionID)
}

func (r *DecreeRepo) AdminArchive(ctx context.Context, versionID uuid.UUID) (*models.DecreeVersion, error) {
	_, err := r.pool.Exec(ctx, `UPDATE decree_versions SET status='archived' WHERE id=$1`, versionID)
	if err != nil {
		return nil, err
	}
	_, _ = r.pool.Exec(ctx, `UPDATE decrees SET current_status='archived' WHERE id = (SELECT decree_id FROM decree_versions WHERE id=$1)`, versionID)
	return r.AdminGetVersion(ctx, versionID)
}

// AdminCreateCorrection inserts a brand-new draft version that supersedes an
// already-published one. The original published row is never touched — this
// is the only sanctioned way to change a published decree's content.
func (r *DecreeRepo) AdminCreateCorrection(ctx context.Context, originalVersionID uuid.UUID, in DecreeInput, createdBy uuid.UUID) (*models.DecreeVersion, error) {
	original, err := r.AdminGetVersion(ctx, originalVersionID)
	if err != nil {
		return nil, err
	}
	if original == nil || original.Status != "published" {
		return nil, ErrInvalidTransition
	}

	nextVersion := original.VersionNumber + 0.1
	hash := contentHash(in.Title, in.FullText, in.IssuingAuthority, fmt.Sprintf("%.1f", nextVersion))

	var versionID uuid.UUID
	err = r.pool.QueryRow(ctx, `
		INSERT INTO decree_versions (decree_id, version_number, title, full_text,
			issuing_authority, integrity_hash, status, is_correction, supersedes_version_id, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, 'draft', true, $7, $8)
		RETURNING id`,
		original.DecreeID, nextVersion, in.Title, in.FullText, in.IssuingAuthority, hash,
		originalVersionID, createdBy).Scan(&versionID)
	if err != nil {
		return nil, err
	}
	return r.AdminGetVersion(ctx, versionID)
}

func contentHash(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0}) // separator so "ab"+"c" can't collide with "a"+"bc"
	}
	return hex.EncodeToString(h.Sum(nil))
}
