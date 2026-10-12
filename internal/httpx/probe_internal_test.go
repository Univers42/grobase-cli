package httpx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

// connectProxy is a minimal HTTP CONNECT proxy that counts the tunnels it opens to target.
func connectProxy(t *testing.T, target string, connects *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect || r.Host != target {
			http.Error(w, "unexpected "+r.Method+" "+r.Host, http.StatusBadGateway)
			return
		}
		up, err := net.Dial("tcp", target)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			_ = up.Close()
			return
		}
		connects.Add(1)
		_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection established\r\n\r\n")
		go func() {
			_, _ = io.Copy(up, conn)
			_ = up.Close()
		}()
		_, _ = io.Copy(conn, up)
		_ = conn.Close()
	}))
}

func TestProbeGoesThroughTheProxyAndSendsNoRequest(t *testing.T) {
	var served, connects atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served.Add(1) }))
	defer srv.Close()
	target := strings.TrimPrefix(srv.URL, "https://")
	proxy := connectProxy(t, target, &connects)
	defer proxy.Close()
	pu, _ := url.Parse(proxy.URL)

	info, err := probeVia(context.Background(), target, http.ProxyURL(pu))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(srv.Certificate().Raw)
	if info.SHA256 != hex.EncodeToString(sum[:]) {
		t.Errorf("probe sha %s, want the server's", info.SHA256)
	}
	if got := connects.Load(); got != 1 {
		t.Errorf("proxy opened %d tunnels, want 1", got)
	}
	if served.Load() != 0 {
		t.Error("the probe sent a request; it must stop after the handshake")
	}
}
