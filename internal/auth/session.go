// Package auth owns login state: GoTrue sessions, the credential Resolver that
// injects headers into requests, and the GoTrue calls the CLI makes on its own behalf.
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Session is a GoTrue user session as stored in the credential store.
type Session struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	UserID       string    `json:"user_id,omitempty"`
	Email        string    `json:"email,omitempty"`
}

// Expiring reports whether the access token is expired or within skew of expiry.
func (s Session) Expiring(now time.Time, skew time.Duration) bool {
	return !s.ExpiresAt.IsZero() && now.Add(skew).After(s.ExpiresAt)
}

// tokenResponse is GoTrue's answer to /token.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	ExpiresAt    int64  `json:"expires_at"`
	User         struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

func (t tokenResponse) session(now time.Time) (Session, error) {
	if t.AccessToken == "" {
		return Session{}, errors.New("server answered without an access token")
	}
	exp := now.Add(time.Duration(t.ExpiresIn) * time.Second)
	if t.ExpiresAt > 0 {
		exp = time.Unix(t.ExpiresAt, 0)
	}
	return Session{
		AccessToken: t.AccessToken, RefreshToken: t.RefreshToken, ExpiresAt: exp,
		UserID: t.User.ID, Email: t.User.Email,
	}, nil
}

// Encode serialises the session for the credential store.
func (s Session) Encode() (string, error) {
	b, err := json.Marshal(s) //nolint:gosec // G117: the session is serialised on purpose, only ever into the encrypted credential store
	if err != nil {
		return "", fmt.Errorf("encode session: %w", err)
	}
	return string(b), nil
}

// DecodeSession parses a stored session.
func DecodeSession(v string) (Session, error) {
	var s Session
	if err := json.Unmarshal([]byte(v), &s); err != nil {
		return Session{}, fmt.Errorf("stored session is corrupt: %w", err)
	}
	return s, nil
}
