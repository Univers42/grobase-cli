package doctor

import (
	"testing"
	"time"

	"github.com/Univers42/grobase-cli/internal/config"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

func TestSkewVerdict(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0: resultPass, 60 * time.Second: resultPass, 61 * time.Second: resultWarn,
		-61 * time.Second: resultWarn, 300 * time.Second: resultWarn, -301 * time.Second: resultFail,
	} {
		if got := skewVerdict(d); got != want {
			t.Errorf("%v: %s, want %s", d, got, want)
		}
	}
}

func TestExpiryVerdict(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for notAfter, want := range map[time.Time]string{
		now.Add(-time.Hour): resultFail, now: resultFail, now.Add(13 * 24 * time.Hour): resultWarn, now.Add(15 * 24 * time.Hour): resultPass,
	} {
		if got, _ := expiryVerdict(notAfter, now); got != want {
			t.Errorf("%v: %s, want %s", notAfter, got, want)
		}
	}
}

func TestTLSVerdict(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	far := now.Add(90 * 24 * time.Hour)
	for name, tc := range map[string]struct {
		c    config.Context
		info httpx.CertInfo
		want string
	}{
		"pin matches":    {config.Context{PinSHA256: "ABCD"}, httpx.CertInfo{SHA256: "abcd", NotAfter: far}, resultPass},
		"pin mismatch":   {config.Context{PinSHA256: "abcd"}, httpx.CertInfo{SHA256: "ffff", NotAfter: far}, resultFail},
		"insecure":       {config.Context{Insecure: true}, httpx.CertInfo{NotAfter: far}, resultWarn},
		"untrusted":      {config.Context{}, httpx.CertInfo{NotAfter: far}, resultWarn},
		"trusted":        {config.Context{}, httpx.CertInfo{Verified: true, NotAfter: far}, resultPass},
		"pinned expired": {config.Context{PinSHA256: "abcd"}, httpx.CertInfo{SHA256: "abcd", NotAfter: now.Add(-time.Hour)}, resultFail},
	} {
		if got, _ := tlsVerdict(tc.c, tc.info, now); got != tc.want {
			t.Errorf("%s: %s, want %s", name, got, tc.want)
		}
	}
}
