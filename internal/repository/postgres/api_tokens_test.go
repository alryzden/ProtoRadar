package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

func TestAPITokenRepositoryCreateGetAndMarkUsed(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repo := NewAPITokenRepository(db)

	expiresAt := time.Date(2026, 6, 8, 10, 30, 0, 0, time.UTC)
	createdAt := time.Date(2026, 6, 7, 10, 30, 0, 0, time.UTC)
	token := domain.APIToken{
		ID:        domain.NewAPITokenID("00000000-0000-0000-0000-000000000101"),
		Name:      "ci-token",
		TokenHash: "sha256:test-token-hash",
		CreatedAt: createdAt,
		ExpiresAt: &expiresAt,
	}

	if err := repo.Create(ctx, token); err != nil {
		t.Fatalf("create token: %v", err)
	}

	byID, err := repo.GetByID(ctx, token.ID)
	if err != nil {
		t.Fatalf("get token by id: %v", err)
	}
	assertAPITokenEqual(t, byID, token)

	byHash, err := repo.GetByHash(ctx, token.TokenHash)
	if err != nil {
		t.Fatalf("get token by hash: %v", err)
	}
	assertAPITokenEqual(t, byHash, token)

	usedAt := time.Date(2026, 6, 7, 11, 0, 0, 0, time.UTC)
	if err := repo.MarkUsed(ctx, token.ID, usedAt); err != nil {
		t.Fatalf("mark token used: %v", err)
	}

	updated, err := repo.GetByID(ctx, token.ID)
	if err != nil {
		t.Fatalf("get updated token: %v", err)
	}
	if updated.LastUsedAt == nil || !updated.LastUsedAt.Equal(usedAt) {
		t.Fatalf("last used at = %v, want %v", updated.LastUsedAt, usedAt)
	}
}

func TestAPITokenRepositoryMarkUsedUnknownTokenReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repo := NewAPITokenRepository(db)

	err := repo.MarkUsed(ctx, domain.NewAPITokenID("00000000-0000-0000-0000-000000000102"), time.Now().UTC())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("mark used error = %v, want %v", err, domain.ErrNotFound)
	}
}

func assertAPITokenEqual(t *testing.T, got domain.APIToken, want domain.APIToken) {
	t.Helper()
	if got.ID != want.ID || got.Name != want.Name || got.TokenHash != want.TokenHash {
		t.Fatalf("token = %#v, want %#v", got, want)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("created at = %v, want %v", got.CreatedAt, want.CreatedAt)
	}
	if (got.ExpiresAt == nil) != (want.ExpiresAt == nil) {
		t.Fatalf("expires at = %v, want %v", got.ExpiresAt, want.ExpiresAt)
	}
	if got.ExpiresAt != nil && !got.ExpiresAt.Equal(*want.ExpiresAt) {
		t.Fatalf("expires at = %v, want %v", got.ExpiresAt, *want.ExpiresAt)
	}
	if got.LastUsedAt != nil || want.LastUsedAt != nil {
		t.Fatalf("last used at = %v, want %v", got.LastUsedAt, want.LastUsedAt)
	}
}
