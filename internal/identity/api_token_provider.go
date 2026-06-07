package identity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

var ErrUnauthenticated = errors.New("unauthenticated")

type Clock interface {
	Now() time.Time
}

type APITokenAuthProvider struct {
	tokens          domain.APITokenRepository
	tokenHashSecret string
	clock           Clock
	observer        TokenUsageObserver
}

func NewAPITokenAuthProvider(tokens domain.APITokenRepository, tokenHashSecret string, clock Clock) *APITokenAuthProvider {
	return NewAPITokenAuthProviderWithObserver(tokens, tokenHashSecret, clock, nil)
}

func NewAPITokenAuthProviderWithObserver(tokens domain.APITokenRepository, tokenHashSecret string, clock Clock, observer TokenUsageObserver) *APITokenAuthProvider {
	return &APITokenAuthProvider{
		tokens:          tokens,
		tokenHashSecret: tokenHashSecret,
		clock:           clock,
		observer:        observer,
	}
}

func (provider *APITokenAuthProvider) Authenticate(ctx context.Context, req AuthRequest) (Principal, error) {
	rawToken, ok := req.Bearer()
	if !ok {
		return Principal{}, ErrUnauthenticated
	}
	if provider == nil || provider.tokens == nil || provider.clock == nil {
		return Principal{}, ErrUnauthenticated
	}

	token, err := provider.tokens.GetByHash(ctx, provider.hashToken(rawToken))
	if errors.Is(err, domain.ErrNotFound) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, err
	}

	now := provider.clock.Now()
	if token.IsExpired(now) {
		return Principal{}, ErrUnauthenticated
	}
	if err := provider.tokens.MarkUsed(ctx, token.ID, now); err != nil {
		provider.recordTokenUsageFailure(ctx, token.ID, err)
	}

	subject := strings.TrimSpace(token.Name)
	if subject == "" {
		subject = token.ID.String()
	}
	return Principal{
		Subject:     subject,
		DisplayName: subject,
		Type:        PrincipalTypeAPIToken,
		Metadata: map[string]string{
			"api_token_id": token.ID.String(),
		},
	}, nil
}

func (provider *APITokenAuthProvider) recordTokenUsageFailure(ctx context.Context, tokenID domain.APITokenID, err error) {
	if provider.observer == nil {
		return
	}
	provider.observer.RecordTokenUsageFailure(ctx, TokenUsageFailure{
		TokenID: tokenID.String(),
		Error:   err,
	})
}

func (req AuthRequest) Bearer() (string, bool) {
	if token := strings.TrimSpace(req.BearerToken); token != "" {
		return token, true
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(req.AuthorizationHeader, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(req.AuthorizationHeader, prefix))
	return token, token != ""
}

func (provider *APITokenAuthProvider) hashToken(rawToken string) string {
	mac := hmac.New(sha256.New, []byte(provider.tokenHashSecret))
	mac.Write([]byte(rawToken))
	return hex.EncodeToString(mac.Sum(nil))
}
