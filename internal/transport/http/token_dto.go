package httptransport

import "time"

type createAPITokenRequest struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expires_at"`
}

type createAPITokenResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Token     string     `json:"token"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}
