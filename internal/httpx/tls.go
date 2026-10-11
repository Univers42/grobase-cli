// Package httpx is the one HTTP client every command uses: TLS trust (CA bundle,
// certificate pin, or explicit insecure mode), credential injection, redacted
// debug tracing and uniform API errors.
package httpx

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// TLS describes how a context trusts the server certificate.
type TLS struct {
	// CAFile is a PEM bundle added to the system roots.
	CAFile string
	// PinSHA256 is the hex SHA-256 of the server's leaf certificate (DER). When set
	// it replaces chain verification: the pinned certificate is the trust anchor,
	// so expiry and hostname are deliberately not checked (self-signed VMs).
	PinSHA256 string
	// Insecure turns verification off entirely.
	Insecure bool
}

// Config builds the *tls.Config for the settings.
func (t TLS) Config() (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	switch {
	case t.PinSHA256 != "":
		want, err := hex.DecodeString(t.PinSHA256)
		if err != nil || len(want) != sha256.Size {
			return nil, errors.New("pin_sha256 must be 64 hex characters")
		}
		cfg.InsecureSkipVerify = true
		cfg.VerifyConnection = pinVerifier(want)
	case t.Insecure:
		cfg.InsecureSkipVerify = true
	case t.CAFile != "":
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		pem, err := os.ReadFile(t.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("CA file %s holds no PEM certificates", t.CAFile)
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}

// pinVerifier uses VerifyConnection (not VerifyPeerCertificate) so the check also
// runs on resumed TLS sessions.
func pinVerifier(want []byte) func(tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
		if len(cs.PeerCertificates) == 0 {
			return errors.New("server presented no certificate")
		}
		got := sha256.Sum256(cs.PeerCertificates[0].Raw)
		if subtle.ConstantTimeCompare(got[:], want) != 1 {
			return fmt.Errorf("certificate pin mismatch: server presented sha256:%s", hex.EncodeToString(got[:]))
		}
		return nil
	}
}

// NewTransport returns an HTTP transport using the TLS settings.
func NewTransport(t TLS, headerTimeout time.Duration) (*http.Transport, error) {
	cfg, err := t.Config()
	if err != nil {
		return nil, err
	}
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("http.DefaultTransport is not an *http.Transport")
	}
	tr := base.Clone()
	tr.TLSClientConfig = cfg
	tr.ResponseHeaderTimeout = headerTimeout
	return tr, nil
}

// CertInfo describes a certificate presented by a server, for trust-on-first-use.
type CertInfo struct {
	SHA256   string
	Subject  string
	Issuer   string
	NotAfter time.Time
	// Verified reports whether the chain verified against the system roots.
	Verified bool
}

// Probe connects to host:port and reports the leaf certificate without trusting it.
// Ponytail: Verified is checked against the system roots only; a CA given by
// --ca-file is not consulted, so a CA-signed server may report Verified=false here
// even though a real request with that CA would succeed.
func Probe(ctx context.Context, hostport string) (CertInfo, error) {
	d := tls.Dialer{Config: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}} //nolint:gosec // SECURITY.md#tls: probing the cert only to show it to the user, nothing is sent
	conn, err := d.DialContext(ctx, "tcp", hostport)
	if err != nil {
		return CertInfo{}, fmt.Errorf("connect %s: %w", hostport, err)
	}
	defer func() { _ = conn.Close() }()
	tc, ok := conn.(*tls.Conn)
	if !ok {
		return CertInfo{}, errors.New("unexpected connection type")
	}
	state := tc.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return CertInfo{}, errors.New("server presented no certificate")
	}
	leaf := state.PeerCertificates[0]
	sum := sha256.Sum256(leaf.Raw)
	host, _, _ := net.SplitHostPort(hostport)
	inter := x509.NewCertPool()
	for _, c := range state.PeerCertificates[1:] {
		inter.AddCert(c)
	}
	_, verr := leaf.Verify(x509.VerifyOptions{DNSName: host, Intermediates: inter})
	return CertInfo{
		SHA256: hex.EncodeToString(sum[:]), Subject: leaf.Subject.String(), Issuer: leaf.Issuer.String(),
		NotAfter: leaf.NotAfter, Verified: verr == nil,
	}, nil
}
