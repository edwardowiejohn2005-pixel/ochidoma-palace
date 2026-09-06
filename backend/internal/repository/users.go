package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ochidoma/platform/internal/models"
)

var ErrTokenInvalid = errors.New("refresh token invalid, expired, or already used")

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

// Pool exposes the underlying pool so callers can pass it to audit.Log
// (or open their own transaction that includes both a mutation and an
// audit write together).
func (r *UserRepo) Pool() *pgxpool.Pool {
	return r.pool
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	err := r.pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.password_hash, u.full_name, u.role_id, ro.name,
		       u.is_active, u.created_at, u.last_login_at
		FROM users u
		JOIN roles ro ON ro.id = u.role_id
		WHERE u.email = $1`, email).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.RoleID, &u.RoleName,
			&u.IsActive, &u.CreatedAt, &u.LastLoginAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var u models.User
	err := r.pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.password_hash, u.full_name, u.role_id, ro.name,
		       u.is_active, u.created_at, u.last_login_at
		FROM users u
		JOIN roles ro ON ro.id = u.role_id
		WHERE u.id = $1`, id).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.RoleID, &u.RoleName,
			&u.IsActive, &u.CreatedAt, &u.LastLoginAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) Create(ctx context.Context, email, passwordHash, fullName string, roleID int) (*models.User, error) {
	var id uuid.UUID
	var createdAt time.Time
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, full_name, role_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`, email, passwordHash, fullName, roleID).
		Scan(&id, &createdAt)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *UserRepo) TouchLastLogin(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, id)
	return err
}

func (r *UserRepo) RoleIDByName(ctx context.Context, name string) (int, error) {
	var id int
	err := r.pool.QueryRow(ctx, `SELECT id FROM roles WHERE name = $1`, name).Scan(&id)
	return id, err
}

// --- refresh tokens ---

func (r *UserRepo) StoreRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time, ip string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at, ip_address)
		VALUES ($1, $2, $3, $4)`, userID, tokenHash, expiresAt, ip)
	return err
}

// ValidateAndRotateRefreshToken looks up a non-revoked, non-expired token by
// hash, revokes it (rotation), and returns the associated user ID.
func (r *UserRepo) ValidateAndRotateRefreshToken(ctx context.Context, tokenHash string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()
		RETURNING user_id`, tokenHash).Scan(&userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return uuid.Nil, ErrTokenInvalid
		}
		return uuid.Nil, err
	}
	return userID, nil
}

func (r *UserRepo) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	return err
}
