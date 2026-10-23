package coinmarketcap

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// recordDelays makes retries wait for nothing and records the delays.
func recordDelays(delays *[]time.Duration) Option {
	var mu sync.Mutex
	return func(c *config) {
		c.sleep = func(ctx context.Context, d time.Duration) error {
			mu.Lock()
			*delays = append(*delays, d)
			mu.Unlock()
			return ctx.Err()
		}
	}
}

// fixedRandom makes the retry jitter deterministic.
func fixedRandom(v float64) Option {
	return func(c *config) { c.random = func() float64 { return v } }
}

// doerFunc adapts a function to HTTPDoer.
type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

// recorded is one request seen by a test server.
type recorded struct {
	Method string
	Path   string
	Query  string
	Header http.Header
	Body   string
}

// testServer answers every request with the next status and body from
// replies (repeating the last one) and records what it received.
type testServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []recorded
}

type reply struct {
	status int
	body   string
	header map[string]string
}

func newTestServer(t *testing.T, replies ...reply) *testServer {
	t.Helper()
	if len(replies) == 0 {
		replies = []reply{{status: http.StatusOK, body: `{}`}}
	}
	s := &testServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		n := len(s.requests)
		s.requests = append(s.requests, recorded{
			Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Header: r.Header.Clone(), Body: string(body),
		})
		s.mu.Unlock()
		rep := replies[min(n, len(replies)-1)]
		for k, v := range rep.header {
			w.Header().Set(k, v)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(rep.status)
		_, _ = io.WriteString(w, rep.body)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *testServer) Requests() []recorded {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]recorded(nil), s.requests...)
}

func mustNew(t *testing.T, opts ...Option) *Client {
	t.Helper()
	c, err := New(opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}
