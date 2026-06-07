package identity

import (
	"context"
	"strings"
)

type PrincipalType string

const (
	PrincipalTypeAPIToken       PrincipalType = "api_token"
	PrincipalTypeUser           PrincipalType = "user"
	PrincipalTypeServiceAccount PrincipalType = "service_account"
	PrincipalTypeSystem         PrincipalType = "system"
)

type Principal struct {
	Subject     string            `json:"subject"`
	DisplayName string            `json:"display_name"`
	Type        PrincipalType     `json:"type"`
	Roles       []string          `json:"roles"`
	Metadata    map[string]string `json:"metadata"`
}

func (principal Principal) IsZero() bool {
	return strings.TrimSpace(principal.Subject) == "" || strings.TrimSpace(principal.Type.String()) == ""
}

func (principalType PrincipalType) String() string {
	return string(principalType)
}

type AuthRequest struct {
	AuthorizationHeader string            `json:"authorization_header"`
	BearerToken         string            `json:"bearer_token"`
	RequestID           string            `json:"request_id"`
	RemoteAddr          string            `json:"remote_addr"`
	Metadata            map[string]string `json:"metadata"`
}

type AuthProvider interface {
	Authenticate(ctx context.Context, req AuthRequest) (Principal, error)
}

type TokenUsageObserver interface {
	RecordTokenUsageFailure(ctx context.Context, failure TokenUsageFailure)
}

type TokenUsageFailure struct {
	TokenID string
	Error   error
}

type principalContextKey struct{}

func ContextWithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	if !ok || principal.IsZero() {
		return Principal{}, false
	}
	return principal, true
}
