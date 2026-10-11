package doctor

import (
	"fmt"
	"strings"
	"time"

	"github.com/Univers42/grobase-cli/internal/config"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// Results a check can report.
const (
	resultPass = "PASS"
	resultWarn = "WARN"
	resultFail = "FAIL"
	resultSkip = "SKIP"
)

// Clock skew thresholds: the server's Date header against the local clock.
const (
	skewWarnAfter = 60 * time.Second
	skewFailAfter = 300 * time.Second
)

// certWarnBefore is how soon a certificate's expiry is worth a warning.
const certWarnBefore = 14 * 24 * time.Hour

// skewVerdict classifies the distance between the local clock and the server's.
// Ponytail: the Date header has one-second resolution and the latency of the
// request counts as skew, so a few seconds of difference is noise, not a broken
// clock on either side.
func skewVerdict(d time.Duration) string {
	d = d.Abs()
	switch {
	case d > skewFailAfter:
		return resultFail
	case d > skewWarnAfter:
		return resultWarn
	}
	return resultPass
}

// expiryVerdict classifies a certificate's remaining lifetime and says how long
// is left. Ponytail: a pinned certificate is its own trust anchor, so no request
// ever checks its expiry; this verdict is reported and never enforced.
func expiryVerdict(notAfter, now time.Time) (string, string) {
	d := notAfter.Sub(now)
	switch {
	case d <= 0:
		return resultFail, fmt.Sprintf("certificate expired on %s", notAfter.Format("2006-01-02"))
	case d < certWarnBefore:
		return resultWarn, fmt.Sprintf("certificate expires on %s (%d days left)", notAfter.Format("2006-01-02"), int(d.Hours()/24))
	}
	return resultPass, fmt.Sprintf("certificate expires on %s", notAfter.Format("2006-01-02"))
}

// tlsVerdict combines how the context trusts the server with the certificate's
// lifetime; the worse of the two wins, so the operator sees the whole picture.
func tlsVerdict(c config.Context, info httpx.CertInfo, now time.Time) (string, string) {
	res, why := trustVerdict(c, info)
	exp, left := expiryVerdict(info.NotAfter, now)
	return worst(res, exp), join(why, left)
}

// trustVerdict reports how the context trusts the server, from the certificate
// the server presented.
func trustVerdict(c config.Context, info httpx.CertInfo) (string, string) {
	switch {
	case c.PinSHA256 != "":
		return pinVerdict(c.PinSHA256, info)
	case c.Insecure:
		return resultWarn, "certificate verification is disabled for this context (--insecure-skip-verify)"
	case c.CAFile != "" && info.Verified:
		return resultPass, "trusted through the system roots and " + c.CAFile
	case c.CAFile != "":
		return resultWarn, "not trusted by the system roots; only " + c.CAFile + " is added to the chain"
	case info.Verified:
		return resultPass, "trusted by the system roots (" + info.Issuer + ")"
	}
	return resultWarn, "not trusted by the system roots: add the CA with `grobase context add --ca-file`, or pin it with `grobase context pin`"
}

// pinVerdict compares the context's pin with the certificate the server
// presented. A mismatch is the one failure that stops every request.
func pinVerdict(pin string, info httpx.CertInfo) (string, string) {
	if strings.EqualFold(pin, info.SHA256) {
		return resultPass, "pin sha256:" + shortHex(pin) + " matches the presented certificate"
	}
	return resultFail, "pin sha256:" + shortHex(pin) + " does not match the presented sha256:" + info.SHA256 +
		" (the certificate was rotated?): run `grobase context pin`"
}

// worst returns the more severe of two results.
func worst(a, b string) string {
	if rank(b) > rank(a) {
		return b
	}
	return a
}

func rank(result string) int {
	switch result {
	case resultFail:
		return 3
	case resultWarn:
		return 2
	case resultPass:
		return 1
	}
	return 0
}

func join(a, b string) string {
	if b == "" {
		return a
	}
	if a == "" {
		return b
	}
	return a + "; " + b
}

// shortHex keeps a fingerprint readable in one table row.
func shortHex(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:12]
}
