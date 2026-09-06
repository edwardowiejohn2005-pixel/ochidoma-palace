package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ochidoma/platform/internal/models"
)

type FoodRepo struct {
	pool *pgxpool.Pool
}

func NewFoodRepo(pool *pgxpool.Pool) *FoodRepo {
	return &FoodRepo{pool: pool}
}

func (r *FoodRepo) Pool() *pgxpool.Pool {
	return r.pool
}

const foodColumns = `id, name, local_name, slug, description, ingredients, preparation,
	cultural_significance, region, status, is_demo_content, created_at`

func (r *FoodRepo) ListPublished(ctx context.Context) ([]models.Food, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+foodColumns+` FROM foods WHERE status = 'published' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Food{}
	for rows.Next() {
		f, err := scanFood(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *FoodRepo) GetBySlug(ctx context.Context, slug string) (*models.Food, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+foodColumns+` FROM foods WHERE slug = $1 AND status = 'published'`, slug)
	f, err := scanFoodRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

// --- admin methods ---

func (r *FoodRepo) AdminList(ctx context.Context, status string) ([]models.Food, error) {
	query := `SELECT ` + foodColumns + ` FROM foods`
	var args []any
	if status != "" {
		query += ` WHERE status = $1`
		args = append(args, status)
	}
	query += ` ORDER BY name`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Food{}
	for rows.Next() {
		f, err := scanFood(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *FoodRepo) AdminGetByID(ctx context.Context, id uuid.UUID) (*models.Food, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+foodColumns+` FROM foods WHERE id = $1`, id)
	f, err := scanFoodRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

type FoodInput struct {
	Name                  string
	LocalName             *string
	Slug                  string
	Description           *string
	Ingredients           *string
	Preparation           *string
	CulturalSignificance  *string
	Region                *string
}

func (r *FoodRepo) AdminCreate(ctx context.Context, in FoodInput) (*models.Food, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO foods (name, local_name, slug, description, ingredients, preparation,
			cultural_significance, region, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'draft')
		RETURNING id`,
		in.Name, in.LocalName, in.Slug, in.Description, in.Ingredients, in.Preparation,
		in.CulturalSignificance, in.Region).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.AdminGetByID(ctx, id)
}

func (r *FoodRepo) AdminUpdate(ctx context.Context, id uuid.UUID, in FoodInput) (*models.Food, error) {
	_, err := r.pool.Exec(ctx, `
		UPDATE foods SET name=$1, local_name=$2, slug=$3, description=$4, ingredients=$5,
			preparation=$6, cultural_significance=$7, region=$8
		WHERE id=$9`,
		in.Name, in.LocalName, in.Slug, in.Description, in.Ingredients, in.Preparation,
		in.CulturalSignificance, in.Region, id)
	if err != nil {
		return nil, err
	}
	return r.AdminGetByID(ctx, id)
}

func (r *FoodRepo) AdminSetStatus(ctx context.Context, id uuid.UUID, status string) (*models.Food, error) {
	_, err := r.pool.Exec(ctx, `UPDATE foods SET status=$1 WHERE id=$2`, status, id)
	if err != nil {
		return nil, err
	}
	return r.AdminGetByID(ctx, id)
}

func (r *FoodRepo) AdminDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM foods WHERE id = $1`, id)
	return err
}

func scanFood(rows pgx.Rows) (models.Food, error) {
	return scanFoodRow(rows)
}

func scanFoodRow(row rowScanner) (models.Food, error) {
	var f models.Food
	err := row.Scan(&f.ID, &f.Name, &f.LocalName, &f.Slug, &f.Description, &f.Ingredients,
		&f.Preparation, &f.CulturalSignificance, &f.Region, &f.Status, &f.IsDemo, &f.CreatedAt)
	return f, err
}
