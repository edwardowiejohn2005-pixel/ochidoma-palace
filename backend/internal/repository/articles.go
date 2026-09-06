package repository

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ochidoma/platform/internal/models"
)

type ArticleRepo struct {
	pool *pgxpool.Pool
}

func NewArticleRepo(pool *pgxpool.Pool) *ArticleRepo {
	return &ArticleRepo{pool: pool}
}

// Pool exposes the underlying pool for use with audit.Log.
func (r *ArticleRepo) Pool() *pgxpool.Pool {
	return r.pool
}

const articleColumns = `id, section, category_id, title, slug, description, body, sources,
	status, is_demo_content, published_at, created_at, updated_at`

// ListPublished returns published articles, optionally filtered by section
// (heritage|history) and/or category_id. Either filter may be zero-value to skip it.
func (r *ArticleRepo) ListPublished(ctx context.Context, section string, categoryID int) ([]models.Article, error) {
	query := `SELECT ` + articleColumns + ` FROM articles WHERE status = 'published'`
	args := []any{}
	argN := 1

	if section != "" {
		query += ` AND section = $` + strconv.Itoa(argN)
		args = append(args, section)
		argN++
	}
	if categoryID != 0 {
		query += ` AND category_id = $` + strconv.Itoa(argN)
		args = append(args, categoryID)
		argN++
	}
	query += ` ORDER BY published_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Article{}
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *ArticleRepo) GetBySlug(ctx context.Context, slug string) (*models.Article, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+articleColumns+` FROM articles WHERE slug = $1 AND status = 'published'`, slug)
	a, err := scanArticleRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// --- admin methods: see drafts/unpublished, and can create/edit/transition status ---

const adminArticleColumns = `id, section, category_id, title, slug, description, body, sources,
	author_id, status, is_demo_content, published_at, created_at, updated_at`

func (r *ArticleRepo) AdminList(ctx context.Context, status string) ([]models.Article, error) {
	query := `SELECT ` + adminArticleColumns + ` FROM articles`
	var args []any
	if status != "" {
		query += ` WHERE status = $1`
		args = append(args, status)
	}
	query += ` ORDER BY updated_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Article{}
	for rows.Next() {
		var a models.Article
		if err := rows.Scan(&a.ID, &a.Section, &a.CategoryID, &a.Title, &a.Slug, &a.Description,
			&a.Body, &a.Sources, &a.AuthorID, &a.Status, &a.IsDemo, &a.PublishedAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *ArticleRepo) AdminGetByID(ctx context.Context, id uuid.UUID) (*models.Article, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+adminArticleColumns+` FROM articles WHERE id = $1`, id)
	var a models.Article
	err := row.Scan(&a.ID, &a.Section, &a.CategoryID, &a.Title, &a.Slug, &a.Description,
		&a.Body, &a.Sources, &a.AuthorID, &a.Status, &a.IsDemo, &a.PublishedAt, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

type ArticleInput struct {
	Section     string
	CategoryID  *int
	Title       string
	Slug        string
	Description *string
	Body        string
	Sources     *string
}

func (r *ArticleRepo) AdminCreate(ctx context.Context, in ArticleInput, authorID uuid.UUID) (*models.Article, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO articles (section, category_id, title, slug, description, body, sources, author_id, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'draft')
		RETURNING id`,
		in.Section, in.CategoryID, in.Title, in.Slug, in.Description, in.Body, in.Sources, authorID).
		Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.AdminGetByID(ctx, id)
}

func (r *ArticleRepo) AdminUpdate(ctx context.Context, id uuid.UUID, in ArticleInput) (*models.Article, error) {
	_, err := r.pool.Exec(ctx, `
		UPDATE articles SET section=$1, category_id=$2, title=$3, slug=$4, description=$5,
			body=$6, sources=$7, updated_at=now()
		WHERE id=$8`,
		in.Section, in.CategoryID, in.Title, in.Slug, in.Description, in.Body, in.Sources, id)
	if err != nil {
		return nil, err
	}
	return r.AdminGetByID(ctx, id)
}

// AdminSetStatus transitions an article's status (draft/in_review/approved/published/archived).
// When moving to 'published' for the first time, published_at is stamped.
func (r *ArticleRepo) AdminSetStatus(ctx context.Context, id uuid.UUID, status string) (*models.Article, error) {
	var err error
	if status == "published" {
		_, err = r.pool.Exec(ctx, `
			UPDATE articles SET status=$1, updated_at=now(),
				published_at = COALESCE(published_at, now())
			WHERE id=$2`, status, id)
	} else {
		_, err = r.pool.Exec(ctx, `UPDATE articles SET status=$1, updated_at=now() WHERE id=$2`, status, id)
	}
	if err != nil {
		return nil, err
	}
	return r.AdminGetByID(ctx, id)
}

func (r *ArticleRepo) AdminDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM articles WHERE id = $1`, id)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanArticle(rows pgx.Rows) (models.Article, error) {
	return scanArticleRow(rows)
}

func scanArticleRow(row rowScanner) (models.Article, error) {
	var a models.Article
	err := row.Scan(&a.ID, &a.Section, &a.CategoryID, &a.Title, &a.Slug, &a.Description,
		&a.Body, &a.Sources, &a.Status, &a.IsDemo, &a.PublishedAt, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

