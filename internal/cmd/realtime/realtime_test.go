package realtime_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/Univers42/grobase-cli/internal/cmd/realtime"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

const anonKey = "anon-rt-123"

type wsServer struct {
	mu       sync.Mutex
	query    string
	received []string
	send     []string
	status   int
}

func (s *wsServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.query = r.URL.RawQuery
	s.mu.Unlock()
	if s.status != 0 {
		w.WriteHeader(s.status)
		return
	}
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = c.CloseNow() }()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	_, b, err := c.Read(ctx)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.received = append(s.received, string(b))
	s.mu.Unlock()
	for _, m := range s.send {
		_ = c.Write(ctx, websocket.MessageText, []byte(m))
	}
	_ = c.Close(websocket.StatusNormalClosure, "")
}

func run(t *testing.T, s *wsServer, args ...string) (int, string) {
	t.Helper()
	h := testenv.New(t, s, map[string]string{secret.KindAnon: anonKey}, "")
	err := h.Run(realtime.New(h.Env), args...)
	return exitcode.Of(err), h.Out.String() + h.Err.String()
}

func TestSubscribeSendsFrameAndPrintsNDJSON(t *testing.T) {
	s := &wsServer{send: []string{`{"n":1}`, `{"n":2}`, `{"n":3}`}}
	code, out := run(t, s, "subscribe", "room", "--event", "msg", "--count", "2", "--timeout", "5s")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if out != "{\"n\":1}\n{\"n\":2}\n" {
		t.Fatalf("stdout = %q", out)
	}
	if len(s.received) != 1 || s.received[0] != `{"channel":"room","event":"msg","type":"subscribe"}` {
		t.Fatalf("frame = %v", s.received)
	}
	if !strings.Contains(s.query, "apikey="+anonKey) {
		t.Fatalf("credentials missing from the dial query: %q", s.query)
	}
}

func TestPublishFrame(t *testing.T) {
	s := &wsServer{}
	if code, out := run(t, s, "publish", "room", "--event", "e", "--data", `{"a":1}`); code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if len(s.received) != 1 || s.received[0] != `{"channel":"room","event":"e","payload":{"a":1},"type":"broadcast"}` {
		t.Fatalf("frame = %v", s.received)
	}
}

func TestHandshakeStatusToExitCode(t *testing.T) {
	for status, want := range map[int]int{401: exitcode.AuthFail, 403: exitcode.AuthFail, 404: exitcode.NotFound, 502: exitcode.Failure} {
		code, out := run(t, &wsServer{status: status}, "subscribe", "room")
		if code != want {
			t.Errorf("%d: exit %d, want %d", status, code, want)
		}
		if strings.Contains(out, anonKey) {
			t.Errorf("%d: credential leaked in %q", status, out)
		}
	}
}

func TestUsage(t *testing.T) {
	for _, args := range [][]string{
		{"subscribe"},
		{"subscribe", " "},
		{"subscribe", "r", "--count", "-1"},
		{"publish", "r"},
		{"publish", "r", "--event", "e"},
		{"publish", "r", "--event", "e", "--data", "{bad"},
	} {
		s := &wsServer{}
		if code, _ := run(t, s, args...); code != exitcode.Usage || len(s.received) != 0 {
			t.Errorf("%v: exit %d", args, code)
		}
	}
}
