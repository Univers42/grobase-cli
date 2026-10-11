package auth

import (
	"context"
	"net/url"
	"time"

	"github.com/Univers42/grobase-cli/internal/httpx"
)

// GoTrue is the slice of the GoTrue API the CLI needs for its own login state.
// Its client must carry no Authorizer; the anon key is passed explicitly.
type GoTrue struct {
	Client *httpx.Client
	Clock  func() time.Time
}

func (g *GoTrue) anonHeader(anon string) map[string][]string {
	if anon == "" {
		return nil
	}
	return map[string][]string{"Apikey": {anon}}
}

func (g *GoTrue) token(ctx context.Context, anon, grant string, body any) (Session, error) {
	var tr tokenResponse
	err := g.Client.JSON(ctx, httpx.Request{
		Method: "POST", Path: "/auth/v1/token", Query: url.Values{"grant_type": {grant}},
		Header: g.anonHeader(anon), JSON: body, Mode: httpx.ModeNone,
	}, &tr)
	if err != nil {
		return Session{}, err
	}
	return tr.session(g.Clock())
}

// Password signs in with email and password.
func (g *GoTrue) Password(ctx context.Context, anon, email, password string) (Session, error) {
	return g.token(ctx, anon, "password", map[string]string{"email": email, "password": password})
}

// Refresh exchanges a refresh token for a new session.
func (g *GoTrue) Refresh(ctx context.Context, anon, refreshToken string) (Session, error) {
	return g.token(ctx, anon, "refresh_token", map[string]string{"refresh_token": refreshToken})
}

// SendOTP asks GoTrue to email a one-time code / magic link to an existing user.
func (g *GoTrue) SendOTP(ctx context.Context, anon, email string) error {
	return g.Client.JSON(ctx, httpx.Request{
		Method: "POST", Path: "/auth/v1/otp", Header: g.anonHeader(anon), Mode: httpx.ModeNone,
		JSON: map[string]any{"email": email, "create_user": false},
	}, nil)
}

// VerifyOTP completes an email OTP login.
func (g *GoTrue) VerifyOTP(ctx context.Context, anon, email, code string) (Session, error) {
	var tr tokenResponse
	err := g.Client.JSON(ctx, httpx.Request{
		Method: "POST", Path: "/auth/v1/verify", Header: g.anonHeader(anon), Mode: httpx.ModeNone,
		JSON: map[string]string{"type": "email", "email": email, "token": code},
	}, &tr)
	if err != nil {
		return Session{}, err
	}
	return tr.session(g.Clock())
}
