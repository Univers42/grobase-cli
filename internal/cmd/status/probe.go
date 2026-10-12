package status

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Univers42/grobase-cli/internal/httpx"
)

// probeTimeout bounds one probe, so a hanging endpoint cannot stall the table.
const probeTimeout = 5 * time.Second

// States reported in the state column.
const (
	stateOK         = "ok"
	stateNotEnabled = "not enabled"
	stateDown       = "down"
)

// noStatus is the status column of a probe that never got an HTTP answer.
const noStatus = "-"

// result is what one probe answered.
type result struct {
	endpoint  string
	status    string
	latencyMS int64
	state     string
	// err is set when no HTTP answer arrived at all (transport failure).
	err error
}

// endpoints lists the reachability probes, in the order they are reported.
func endpoints() []string {
	return []string{
		"/auth/v1/health",
		"/rest/v1/",
		// /storage/v1/version is not a reachability signal (it is absent from
		// deployments that serve storage just fine), /storage/v1/bucket answers
		// 401 as soon as the service is there and wants a key.
		"/storage/v1/bucket",
		"/query/v1/engines",
		// Any HTTP answer at all is enough for the realtime gateway.
		"/realtime/v1/",
		"/functions/v1/",
		"/v1/tenants/me",
	}
}

// probeAll runs every probe concurrently and keeps the declared order.
func probeAll(ctx context.Context, cl *httpx.Client, paths []string) []result {
	out := make([]result, len(paths))
	var wg sync.WaitGroup
	for i, path := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = probe(ctx, cl, path)
		}()
	}
	wg.Wait()
	return out
}

func probe(ctx context.Context, cl *httpx.Client, path string) result {
	pctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	start := time.Now()
	resp, err := cl.Do(pctx, httpx.Request{Method: http.MethodGet, Path: path, Mode: httpx.ModeNone})
	r := result{endpoint: path, latencyMS: time.Since(start).Milliseconds()}
	if err == nil {
		_ = resp.Body.Close()
		r.status, r.state = strconv.Itoa(resp.StatusCode), state(resp.StatusCode)
		return r
	}
	var apiErr *httpx.APIError
	if !errors.As(err, &apiErr) {
		r.status, r.state, r.err = noStatus, stateDown, err
		return r
	}
	r.status, r.state = strconv.Itoa(apiErr.Status), state(apiErr.Status)
	if apiErr.NotFound() {
		r.state = apiErr.Miss().FeatureState()
	}
	return r
}

// state turns a status into the reported state. A 404 means the deployment does not
// carry the feature (probe refines it into not deployed or switched off when the body
// says which) and a 5xx that it is broken; anything else, 401 and 403 included, proves
// the route is served.
func state(code int) string {
	switch {
	case code == http.StatusNotFound:
		return stateNotEnabled
	case code >= http.StatusInternalServerError:
		return stateDown
	}
	return stateOK
}

func rows(results []result) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		out = append(out, map[string]any{
			"endpoint": r.endpoint, "status": r.status, "latency_ms": r.latencyMS, "state": r.state,
		})
	}
	return out
}

// transportError returns the first transport failure, but only when every probe
// failed: if any endpoint answered, the context URL itself is reachable.
func transportError(results []result) error {
	var first error
	for _, r := range results {
		if r.err == nil {
			return nil
		}
		if first == nil {
			first = r.err
		}
	}
	return first
}
