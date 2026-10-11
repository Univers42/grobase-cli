package realtime_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/Univers42/grobase-cli/internal/auth"
	"github.com/Univers42/grobase-cli/internal/cmd/realtime"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

const (
	anonKey = "anon-rt-123"
	userJWT = "user-jwt-rt-456"
)

// wsServer is a fake realtime gateway: it records the frames the CLI sends and
// answers from a script, one entry per received frame, AUTH first like the
// protocol requires.
type wsServer struct {
	mu       sync.Mutex
	query    string
	accepted int
	received []string
	status   int
	steps    [][]string
}

func (s *wsServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.query = r.URL.RawQuery
	s.accepted++
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
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	for i := 0; ; i++ {
		_, b, err := c.Read(ctx)
		if err != nil {
			return
		}
		s.mu.Lock()
		s.received = append(s.received, string(b))
		var reply []string
		if i < len(s.steps) {
			reply = s.steps[i]
		}
		s.mu.Unlock()
		for _, m := range reply {
			if err := c.Write(ctx, websocket.MessageText, []byte(m)); err != nil {
				return
			}
		}
	}
}

// frames returns the frames the CLI sent, in order.
func (s *wsServer) frames() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.received...)
}

// queryOf returns the query of the dial request.
func (s *wsServer) queryOf() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.query
}

// acceptedCount returns how many connections reached the handler.
func (s *wsServer) acceptedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.accepted
}

// userCreds is a logged-in user: the session authenticates the socket and the
// anon key rides along as apikey.
func userCreds(t *testing.T) map[string]string {
	t.Helper()
	s := auth.Session{
		AccessToken:  userJWT,
		RefreshToken: "refresh-rt",
		ExpiresAt:    time.Date(2026, 1, 2, 4, 4, 5, 0, time.UTC),
	}
	enc, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{secret.KindAnon: anonKey, secret.KindSession: enc}
}

// run executes the command against s and reports the exit code, the error text
// (empty on success), stdout and stderr.
func run(t *testing.T, s *wsServer, creds map[string]string, args ...string) (int, string, string, string) {
	t.Helper()
	h := testenv.New(t, s, creds, "")
	err := h.Run(realtime.New(h.Env), args...)
	errText := ""
	if err != nil {
		errText = err.Error()
	}
	return exitcode.Of(err), errText, h.Out.String(), h.Err.String()
}

// eventSteps is the gateway script of the happy paths: AUTH_OK, then SUBSCRIBED
// followed by a broadcast, an insert and another broadcast.
func eventSteps() [][]string {
	return [][]string{
		{`{"type":"AUTH_OK","conn_id":"c1","server_time":"2026-01-02T03:04:05Z"}`},
		{
			`{"type":"SUBSCRIBED","sub_id":"cli-1","seq":1}`,
			`{"type":"EVENT","sub_id":"cli-1","event":{"topic":"room","event_type":"broadcast","payload":{"event":"ping","n":1}}}`,
			`{"type":"EVENT","sub_id":"cli-1","event":{"topic":"room","event_type":"insert","payload":{"id":7}}}`,
			`{"type":"EVENT","sub_id":"cli-1","event":{"topic":"room","event_type":"broadcast","payload":{"event":"ping","n":2}}}`,
		},
	}
}

func TestSubscribeAuthenticatesThenSubscribes(t *testing.T) {
	s := &wsServer{steps: eventSteps()}
	code, errText, stdout, _ := run(t, s, userCreds(t), "subscribe", "room", "--count", "2", "--timeout", "5s")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errText)
	}
	want := "{\"topic\":\"room\",\"event\":\"ping\",\"payload\":{\"event\":\"ping\",\"n\":1}}\n" +
		"{\"topic\":\"room\",\"event\":\"insert\",\"payload\":{\"id\":7}}\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	f := s.frames()
	if len(f) != 2 {
		t.Fatalf("frames = %v", f)
	}
	var auth, sub map[string]any
	if json.Unmarshal([]byte(f[0]), &auth) != nil || auth["type"] != "AUTH" || auth["token"] != userJWT {
		t.Fatalf("first frame = %s", f[0])
	}
	if json.Unmarshal([]byte(f[1]), &sub) != nil || sub["type"] != "SUBSCRIBE" ||
		sub["sub_id"] != "cli-1" || sub["topic"] != "room" {
		t.Fatalf("second frame = %s", f[1])
	}
	q := s.queryOf()
	if !strings.Contains(q, "apikey="+anonKey) || strings.Contains(q, "access_token") || strings.Contains(q, userJWT) {
		t.Fatalf("dial query = %q", q)
	}
}

func TestSubscribeEventFilter(t *testing.T) {
	s := &wsServer{steps: eventSteps()}
	code, errText, stdout, stderr := run(t, s, userCreds(t), "subscribe", "room",
		"--event", "ping", "--count", "1", "--timeout", "5s")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errText)
	}
	want := "{\"topic\":\"room\",\"event\":\"ping\",\"payload\":{\"event\":\"ping\",\"n\":1}}\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestSubscribeDebugPrintsSubscribed(t *testing.T) {
	s := &wsServer{steps: eventSteps()}
	h := testenv.New(t, s, userCreds(t), "")
	h.Env.Flags.Debug = true
	if err := h.Run(realtime.New(h.Env), "subscribe", "room", "--count", "1", "--timeout", "5s"); err != nil {
		t.Fatalf("%v (stderr: %s)", err, h.Err)
	}
	if !strings.Contains(h.Err.String(), "subscribed to room") {
		t.Fatalf("stderr = %q", h.Err.String())
	}
}

func TestAuthFailed(t *testing.T) {
	s := &wsServer{steps: [][]string{{`{"type":"ERROR","code":"AUTH_FAILED","message":"bad token"}`}}}
	code, errText, _, _ := run(t, s, userCreds(t), "subscribe", "room", "--timeout", "5s")
	if code != exitcode.AuthFail {
		t.Fatalf("exit %d, want %d: %s", code, exitcode.AuthFail, errText)
	}
	if !strings.Contains(errText, "AUTH_FAILED") {
		t.Fatalf("error = %q", errText)
	}
	if f := s.frames(); len(f) != 1 {
		t.Fatalf("frames = %v", f)
	}
}

func TestPublish(t *testing.T) {
	s := &wsServer{steps: [][]string{
		{`{"type":"AUTH_OK","conn_id":"c1","server_time":"2026-01-02T03:04:05Z"}`},
		{},
	}}
	code, errText, stdout, _ := run(t, s, userCreds(t), "publish", "room", "--event", "e", "--data", `{"a":1}`)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errText)
	}
	if !strings.Contains(stdout, `"channel": "room"`) || !strings.Contains(stdout, `"event": "e"`) {
		t.Fatalf("stdout = %q", stdout)
	}
	f := s.frames()
	if len(f) != 2 {
		t.Fatalf("frames = %v", f)
	}
	var auth, bc map[string]any
	if json.Unmarshal([]byte(f[0]), &auth) != nil || auth["type"] != "AUTH" || auth["token"] != userJWT {
		t.Fatalf("first frame = %s", f[0])
	}
	if json.Unmarshal([]byte(f[1]), &bc) != nil || bc["type"] != "BROADCAST" ||
		bc["topic"] != "room" || bc["event"] != "e" {
		t.Fatalf("second frame = %s", f[1])
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(mustJSON(t, bc["payload"])), &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if len(payload) != 1 || payload["a"] != float64(1) {
		t.Fatalf("payload = %v", payload)
	}
}

// mustJSON re-encodes v so the payload can be compared as JSON text.
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAnonContextNeedsLogin(t *testing.T) {
	s := &wsServer{}
	code, errText, _, _ := run(t, s, map[string]string{secret.KindAnon: anonKey},
		"subscribe", "room", "--timeout", "5s")
	if code != exitcode.Usage {
		t.Fatalf("exit %d, want %d: %s", code, exitcode.Usage, errText)
	}
	if !strings.Contains(errText, "grobase login") {
		t.Fatalf("error = %q", errText)
	}
	if n := s.acceptedCount(); n != 0 {
		t.Fatalf("connections = %d, want 0", n)
	}
}

func TestHandshakeStatusToExitCode(t *testing.T) {
	for status, want := range map[int]int{401: exitcode.AuthFail, 403: exitcode.AuthFail, 404: exitcode.NotFound, 502: exitcode.Failure} {
		code, errText, _, _ := run(t, &wsServer{status: status}, userCreds(t), "subscribe", "room")
		if code != want {
			t.Errorf("%d: exit %d, want %d", status, code, want)
		}
		if strings.Contains(errText, anonKey) || strings.Contains(errText, userJWT) {
			t.Errorf("%d: credential leaked in %q", status, errText)
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
		code, _, _, _ := run(t, s, userCreds(t), args...)
		if code != exitcode.Usage {
			t.Errorf("%v: exit %d", args, code)
		}
		if n := s.acceptedCount(); n != 0 {
			t.Errorf("%v: %d connections, want 0", args, n)
		}
	}
}
