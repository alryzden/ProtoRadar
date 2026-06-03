package domain

import "time"

type APIToken struct {
	ID         APITokenID
	Name       string
	TokenHash  string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
}

func (token APIToken) IsExpired(now time.Time) bool {
	return token.ExpiresAt != nil && !token.ExpiresAt.After(now)
}
