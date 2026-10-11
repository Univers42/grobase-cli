package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Univers42/grobase-cli/internal/httpx"
	"github.com/Univers42/grobase-cli/internal/redact"
	"github.com/Univers42/grobase-cli/internal/secret"
)

// ErrNoCredential means the requested credential kind is not stored for the context.
var ErrNoCredential = errors.New("no credential")

const refreshSkew = 60 * time.Second

// Resolver implements httpx.Authorizer from the credential store of one context.
type Resolver struct {
	Context string
	Store   secret.Store
	Clock   func() time.Time
	Redact  *redact.Redactor
	// Gate performs session refreshes.
	Gate *GoTrue

	mu sync.Mutex
}

func (r *Resolver) load(kind string) (string, error) {
	v, err := r.Store.Get(secret.Name(r.Context, kind))
	if errors.Is(err, secret.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s credential: %w", kind, err)
	}
	r.Redact.Add(v)
	return v, nil
}

// Session returns the stored session, refreshing it first when about to expire.
func (r *Resolver) Session(ctx context.Context) (Session, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	raw, err := r.load(secret.KindSession)
	if err != nil || raw == "" {
		return Session{}, false, err
	}
	s, err := DecodeSession(raw)
	if err != nil {
		return Session{}, false, err
	}
	r.Redact.Add(s.AccessToken)
	r.Redact.Add(s.RefreshToken)
	if s.Expiring(r.Clock(), refreshSkew) && s.RefreshToken != "" {
		if ns, rerr := r.renew(ctx, s); rerr == nil {
			s = ns
		} else if s.Expiring(r.Clock(), 0) {
			return Session{}, false, fmt.Errorf("session expired and refresh failed (run `grobase login`): %w", rerr)
		}
	}
	return s, true, nil
}

func (r *Resolver) renew(ctx context.Context, s Session) (Session, error) {
	anon, err := r.load(secret.KindAnon)
	if err != nil {
		return Session{}, err
	}
	ns, err := r.Gate.Refresh(ctx, anon, s.RefreshToken)
	if err != nil {
		return Session{}, err
	}
	enc, err := ns.Encode()
	if err != nil {
		return Session{}, err
	}
	if err := r.Store.Set(secret.Name(r.Context, secret.KindSession), enc); err != nil {
		return Session{}, fmt.Errorf("save refreshed session: %w", err)
	}
	r.Redact.Add(ns.AccessToken)
	r.Redact.Add(ns.RefreshToken)
	return ns, nil
}

// Refresh implements httpx.Authorizer: force a renewal after a 401.
func (r *Resolver) Refresh(ctx context.Context) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	raw, err := r.load(secret.KindSession)
	if err != nil || raw == "" {
		return false, err
	}
	s, err := DecodeSession(raw)
	if err != nil || s.RefreshToken == "" {
		return false, err
	}
	_, err = r.renew(ctx, s)
	return err == nil, err
}

func need(kind, v string) (string, error) {
	if v == "" {
		return "", fmt.Errorf("%w: %s is not set for this context (see `grobase login --help`)", ErrNoCredential, kind)
	}
	return v, nil
}

// Authorize implements httpx.Authorizer.
func (r *Resolver) Authorize(ctx context.Context, req *http.Request, mode httpx.Mode) error {
	if mode == httpx.ModeAuto {
		mode = r.pick(ctx)
	}
	apikey, bearer, err := r.credentials(ctx, mode)
	if err != nil {
		return err
	}
	if apikey != "" {
		req.Header.Set("apikey", apikey)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	return nil
}

// pick chooses the strongest stored credential for ModeAuto.
func (r *Resolver) pick(ctx context.Context) httpx.Mode {
	if _, ok, _ := r.Session(ctx); ok {
		return httpx.ModeUser
	}
	for _, c := range []struct {
		kind string
		mode httpx.Mode
	}{{secret.KindAPIKey, httpx.ModeAPIKey}, {secret.KindServiceRole, httpx.ModeService}} {
		if v, _ := r.load(c.kind); v != "" {
			return c.mode
		}
	}
	return httpx.ModeAnon
}

func (r *Resolver) credentials(ctx context.Context, mode httpx.Mode) (apikey, bearer string, err error) {
	switch mode {
	case httpx.ModeService:
		k, err := r.load(secret.KindServiceRole)
		if err == nil {
			k, err = need("service-role key", k)
		}
		return k, k, err
	case httpx.ModeAnon:
		k, err := r.load(secret.KindAnon)
		if err == nil {
			k, err = need("anon key", k)
		}
		return k, k, err
	case httpx.ModeAPIKey:
		anon, _ := r.load(secret.KindAnon)
		k, err := r.load(secret.KindAPIKey)
		if err == nil {
			k, err = need("API key", k)
		}
		return anon, k, err
	default:
		anon, _ := r.load(secret.KindAnon)
		s, ok, err := r.Session(ctx)
		if err == nil && !ok {
			err = fmt.Errorf("%w: not logged in (run `grobase login`)", ErrNoCredential)
		}
		return anon, s.AccessToken, err
	}
}
