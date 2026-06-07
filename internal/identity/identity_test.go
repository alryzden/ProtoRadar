package identity

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

func TestPrincipalContextRoundTrip(t *testing.T) {
	principal := Principal{
		Subject:     "ci",
		DisplayName: "CI token",
		Type:        PrincipalTypeAPIToken,
		Roles:       []string{"publisher"},
		Metadata:    map[string]string{"api_token_id": "token-1"},
	}

	got, ok := PrincipalFromContext(ContextWithPrincipal(context.Background(), principal))
	if !ok {
		t.Fatalf("principal not found")
	}
	if got.Subject != principal.Subject || got.Type != principal.Type || got.Metadata["api_token_id"] != "token-1" {
		t.Fatalf("principal = %#v", got)
	}
}

func TestPrincipalFromContextMissing(t *testing.T) {
	if principal, ok := PrincipalFromContext(context.Background()); ok {
		t.Fatalf("principal = %#v, want missing", principal)
	}
}

func TestAPITokenAuthProviderAuthenticatesValidToken(t *testing.T) {
	clock := fixedClock{now: time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)}
	tokens := newFakeTokenRepository()
	provider := NewAPITokenAuthProvider(tokens, "hash-secret", clock)
	tokens.store(t, provider, domain.APIToken{
		ID:        domain.NewAPITokenID("token-1"),
		Name:      "ci",
		CreatedAt: clock.now,
	}, "raw-token")

	principal, err := provider.Authenticate(context.Background(), AuthRequest{AuthorizationHeader: "Bearer raw-token"})
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if principal.Type != PrincipalTypeAPIToken || principal.Subject != "ci" || principal.Metadata["api_token_id"] != "token-1" {
		t.Fatalf("principal = %#v", principal)
	}
	if tokens.lastUsed["token-1"].IsZero() {
		t.Fatalf("last_used_at was not updated")
	}
}

func TestAPITokenAuthProviderAuthenticatesWhenMarkUsedFailsAndReportsFailure(t *testing.T) {
	clock := fixedClock{now: time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)}
	tokens := newFakeTokenRepository()
	observer := &fakeTokenUsageObserver{}
	provider := NewAPITokenAuthProviderWithObserver(tokens, "hash-secret", clock, observer)
	tokens.markUsedErr = errors.New("mark used failed")
	tokens.store(t, provider, domain.APIToken{
		ID:        domain.NewAPITokenID("token-1"),
		Name:      "ci",
		CreatedAt: clock.now,
	}, "raw-token")

	principal, err := provider.Authenticate(context.Background(), AuthRequest{AuthorizationHeader: "Bearer raw-token"})
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if principal.Subject != "ci" || principal.Metadata["api_token_id"] != "token-1" {
		t.Fatalf("principal = %#v", principal)
	}
	if len(observer.failures) != 1 {
		t.Fatalf("token usage failures = %#v, want 1", observer.failures)
	}
	if observer.failures[0].TokenID != "token-1" {
		t.Fatalf("token id = %q, want token-1", observer.failures[0].TokenID)
	}
	if !errors.Is(observer.failures[0].Error, tokens.markUsedErr) {
		t.Fatalf("failure error = %v, want mark error", observer.failures[0].Error)
	}
	if strings.Contains(observer.failures[0].TokenID, "raw-token") {
		t.Fatalf("observer leaked raw token: %#v", observer.failures[0])
	}
}

func TestAPITokenAuthProviderRejectsInvalidToken(t *testing.T) {
	provider := NewAPITokenAuthProvider(newFakeTokenRepository(), "hash-secret", fixedClock{now: time.Now()})

	_, err := provider.Authenticate(context.Background(), AuthRequest{BearerToken: "wrong-token"})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("error = %v, want ErrUnauthenticated", err)
	}
}

func TestAPITokenAuthProviderRejectsMissingToken(t *testing.T) {
	provider := NewAPITokenAuthProvider(newFakeTokenRepository(), "hash-secret", fixedClock{now: time.Now()})

	_, err := provider.Authenticate(context.Background(), AuthRequest{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("error = %v, want ErrUnauthenticated", err)
	}
}

func TestAPITokenAuthProviderRejectsExpiredToken(t *testing.T) {
	clock := fixedClock{now: time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)}
	tokens := newFakeTokenRepository()
	provider := NewAPITokenAuthProvider(tokens, "hash-secret", clock)
	expiresAt := clock.now.Add(-time.Hour)
	tokens.store(t, provider, domain.APIToken{
		ID:        domain.NewAPITokenID("token-1"),
		Name:      "expired",
		CreatedAt: clock.now.Add(-2 * time.Hour),
		ExpiresAt: &expiresAt,
	}, "raw-token")

	_, err := provider.Authenticate(context.Background(), AuthRequest{BearerToken: "raw-token"})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("error = %v, want ErrUnauthenticated", err)
	}
}

func TestAPITokenAuthProviderErrorsDoNotContainRawToken(t *testing.T) {
	provider := NewAPITokenAuthProvider(newFakeTokenRepository(), "hash-secret", fixedClock{now: time.Now()})

	_, err := provider.Authenticate(context.Background(), AuthRequest{BearerToken: "secret-token"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("error leaked raw token: %v", err)
	}
}

type fixedClock struct {
	now time.Time
}

func (clock fixedClock) Now() time.Time {
	return clock.now
}

type fakeTokenRepository struct {
	byHash      map[string]domain.APIToken
	lastUsed    map[string]time.Time
	markUsedErr error
}

func newFakeTokenRepository() *fakeTokenRepository {
	return &fakeTokenRepository{
		byHash:   map[string]domain.APIToken{},
		lastUsed: map[string]time.Time{},
	}
}

func (repo *fakeTokenRepository) store(t *testing.T, provider *APITokenAuthProvider, token domain.APIToken, rawToken string) {
	t.Helper()
	token.TokenHash = provider.hashToken(rawToken)
	if err := repo.Create(context.Background(), token); err != nil {
		t.Fatalf("store token: %v", err)
	}
}

func (repo *fakeTokenRepository) Create(ctx context.Context, token domain.APIToken) error {
	repo.byHash[token.TokenHash] = token
	return nil
}

func (repo *fakeTokenRepository) GetByID(ctx context.Context, id domain.APITokenID) (domain.APIToken, error) {
	for _, token := range repo.byHash {
		if token.ID == id {
			return token, nil
		}
	}
	return domain.APIToken{}, domain.ErrNotFound
}

func (repo *fakeTokenRepository) GetByHash(ctx context.Context, tokenHash string) (domain.APIToken, error) {
	token, ok := repo.byHash[tokenHash]
	if !ok {
		return domain.APIToken{}, domain.ErrNotFound
	}
	return token, nil
}

func (repo *fakeTokenRepository) MarkUsed(ctx context.Context, id domain.APITokenID, usedAt time.Time) error {
	if repo.markUsedErr != nil {
		return repo.markUsedErr
	}
	repo.lastUsed[id.String()] = usedAt
	return nil
}

type fakeTokenUsageObserver struct {
	failures []TokenUsageFailure
}

func (observer *fakeTokenUsageObserver) RecordTokenUsageFailure(ctx context.Context, failure TokenUsageFailure) {
	observer.failures = append(observer.failures, failure)
}
