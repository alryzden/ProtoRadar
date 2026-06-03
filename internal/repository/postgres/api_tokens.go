package postgres

import (
	"context"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type APITokenRepository struct {
	db *DB
}

func NewAPITokenRepository(db *DB) *APITokenRepository {
	return &APITokenRepository{db: db}
}

func (repo *APITokenRepository) Create(ctx context.Context, token domain.APIToken) error {
	_, err := repo.db.executor(ctx).Exec(ctx, `
		INSERT INTO api_tokens (id, name, token_hash, created_at, expires_at, last_used_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		token.ID.String(),
		token.Name,
		token.TokenHash,
		token.CreatedAt,
		nullableTime(token.ExpiresAt),
		nullableTime(token.LastUsedAt),
	)
	return mapError(err)
}

func (repo *APITokenRepository) GetByID(ctx context.Context, id domain.APITokenID) (domain.APIToken, error) {
	return repo.getOne(ctx, `
		SELECT id, name, token_hash, created_at, expires_at, last_used_at
		FROM api_tokens
		WHERE id = $1
	`, id.String())
}

func (repo *APITokenRepository) GetByHash(ctx context.Context, tokenHash string) (domain.APIToken, error) {
	return repo.getOne(ctx, `
		SELECT id, name, token_hash, created_at, expires_at, last_used_at
		FROM api_tokens
		WHERE token_hash = $1
	`, tokenHash)
}

func (repo *APITokenRepository) MarkUsed(ctx context.Context, id domain.APITokenID, usedAt time.Time) error {
	tag, err := repo.db.executor(ctx).Exec(ctx, `
		UPDATE api_tokens
		SET last_used_at = $2
		WHERE id = $1
	`, id.String(), usedAt)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (repo *APITokenRepository) getOne(ctx context.Context, query string, args ...any) (domain.APIToken, error) {
	var token domain.APIToken
	var id string

	err := repo.db.executor(ctx).QueryRow(ctx, query, args...).Scan(
		&id,
		&token.Name,
		&token.TokenHash,
		&token.CreatedAt,
		&token.ExpiresAt,
		&token.LastUsedAt,
	)
	if err != nil {
		return domain.APIToken{}, mapError(err)
	}

	token.ID = domain.NewAPITokenID(id)
	return token, nil
}

var _ domain.APITokenRepository = (*APITokenRepository)(nil)
