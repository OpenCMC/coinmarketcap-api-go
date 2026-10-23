package coinmarketcap

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryDelay(t *testing.T) {
	header := func(retryAfter string) http.Header {
		h := http.Header{}
		if retryAfter != "" {
			h.Set("Retry-After", retryAfter)
		}
		return h
	}
	tests := []struct {
		name       string
		attempt    int
		status     int
		retryAfter string
		random     float64
		want       time.Duration
	}{
		{"first retry", 0, 503, "", 0, 500 * time.Millisecond},
		{"second retry", 1, 503, "", 0, time.Second},
		{"third retry", 2, 503, "", 0, 2 * time.Second},
		{"jitter is at most 20%", 0, 503, "", 0.5, 550 * time.Millisecond},
		{"capped at 8s", 5, 503, "", 0.9, 8 * time.Second},
		{"network error", 1, 0, "", 0, time.Second},
		{"429 Retry-After", 0, 429, "2", 0, 2 * time.Second},
		{"429 fractional Retry-After", 3, 429, "0.5", 0, 500 * time.Millisecond},
		{"429 Retry-After capped", 0, 429, "3600", 0, 8 * time.Second},
		{"429 negative Retry-After", 0, 429, "-5", 0, 0},
		{"429 HTTP-date Retry-After falls back to backoff", 1, 429, "Wed, 21 Oct 2015 07:28:00 GMT", 0, time.Second},
		{"429 NaN Retry-After falls back to backoff", 0, 429, "NaN", 0, 500 * time.Millisecond},
		{"Retry-After only counts on 429", 0, 503, "2", 0, 500 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := retryDelay(tt.attempt, tt.status, header(tt.retryAfter), tt.random); got != tt.want {
				t.Errorf("retryDelay = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRetriesRetryableStatusesThenSucceeds(t *testing.T) {
	for _, status := range []int{408, 409, 429, 500, 502, 503, 504} {
		srv := newTestServer(t, reply{status: status, body: `{}`}, reply{status: status, body: `{}`}, reply{status: 200, body: `{}`})
		var delays []time.Duration
		cmc := mustNew(t, WithAPIKey("key"), WithBaseURL(srv.URL), recordDelays(&delays), fixedRandom(0))

		if _, err := cmc.GlobalMetrics.QuotesLatest(context.Background(), nil); err != nil {
			t.Fatalf("status %d: %v", status, err)
		}
		if n := len(srv.Requests()); n != 3 {
			t.Errorf("status %d: %d requests, want 3", status, n)
		}
		if want := []time.Duration{500 * time.Millisecond, time.Second}; !reflect.DeepEqual(delays, want) {
			t.Errorf("status %d: delays %v, want %v", status, delays, want)
		}
	}
}

func TestGivesUpAfterMaxRetries(t *testing.T) {
	srv := newTestServer(t, reply{status: 503, body: `{"status":{"error_code":503,"error_message":"busy"}}`})
	var delays []time.Duration
	cmc := mustNew(t, WithAPIKey("key"), WithBaseURL(srv.URL), recordDelays(&delays))

	_, err := cmc.GlobalMetrics.QuotesLatest(context.Background(), nil)
	var serverErr *InternalServerError
	if !errors.As(err, &serverErr) || serverErr.Message != "busy" {
		t.Fatalf("error = %v, want *InternalServerError with the API message", err)
	}
	if n := len(srv.Requests()); n != 3 {
		t.Errorf("%d requests, want 3 (1 + 2 retries)", n)
	}
}

func TestMaxRetriesZeroDisablesRetries(t *testing.T) {
	srv := newTestServer(t, reply{status: 503, body: `{}`})
	cmc := mustNew(t, WithAPIKey("key"), WithBaseURL(srv.URL), WithMaxRetries(0))
	if _, err := cmc.GlobalMetrics.QuotesLatest(context.Background(), nil); err == nil {
		t.Fatal("expected an error")
	}
	if n := len(srv.Requests()); n != 1 {
		t.Errorf("%d requests, want 1", n)
	}
}

func TestDoesNotRetryOtherStatuses(t *testing.T) {
	for _, status := range []int{400, 401, 402, 403, 404, 501} {
		srv := newTestServer(t, reply{status: status, body: `{}`})
		cmc := mustNew(t, WithAPIKey("key"), WithBaseURL(srv.URL), recordDelays(new([]time.Duration)))
		if _, err := cmc.GlobalMetrics.QuotesLatest(context.Background(), nil); err == nil {
			t.Fatalf("status %d: expected an error", status)
		}
		if n := len(srv.Requests()); n != 1 {
			t.Errorf("status %d: %d requests, want 1", status, n)
		}
	}
}

func TestRateLimitHonoursRetryAfter(t *testing.T) {
	srv := newTestServer(t,
		reply{status: 429, body: `{}`, header: map[string]string{"Retry-After": "3"}},
		reply{status: 429, body: `{}`, header: map[string]string{"Retry-After": "3600"}},
		reply{status: 200, body: `{}`},
	)
	var delays []time.Duration
	cmc := mustNew(t, WithAPIKey("key"), WithBaseURL(srv.URL), recordDelays(&delays))
	if _, err := cmc.GlobalMetrics.QuotesLatest(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if want := []time.Duration{3 * time.Second, 8 * time.Second}; !reflect.DeepEqual(delays, want) {
		t.Errorf("delays %v, want %v", delays, want)
	}
}

func TestPOSTBodyIsReplayedOnRetry(t *testing.T) {
	srv := newTestServer(t, reply{status: 503, body: `{}`}, reply{status: 200, body: `{}`})
	cmc := mustNew(t, WithAPIKey("key"), WithBaseURL(srv.URL), recordDelays(new([]time.Duration)))
	body := TokenBatchQueryTokensBody{Platform: String("ethereum"), Addresses: &[]string{"0xa0b8"}}

	if _, err := cmc.Token.BatchQueryTokens(context.Background(), body); err != nil {
		t.Fatal(err)
	}
	reqs := srv.Requests()
	if len(reqs) != 2 {
		t.Fatalf("%d requests, want 2", len(reqs))
	}
	if reqs[0].Body == "" || reqs[0].Body != reqs[1].Body {
		t.Errorf("bodies differ between attempts: %q vs %q", reqs[0].Body, reqs[1].Body)
	}
}

func TestBodyWithoutGetBodyIsReplayed(t *testing.T) {
	srv := newTestServer(t, reply{status: 502, body: `{}`}, reply{status: 200, body: `{}`})
	d := &retryingDoer{client: http.DefaultClient, maxRetries: 1, sleep: func(context.Context, time.Duration) error { return nil }}

	// io.NopCloser hides the reader type, so http.NewRequest sets no GetBody.
	req, err := http.NewRequest(http.MethodPost, srv.URL, io.NopCloser(strings.NewReader("payload")))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := d.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	for i, r := range srv.Requests() {
		if r.Body != "payload" {
			t.Errorf("attempt %d body = %q, want payload", i+1, r.Body)
		}
	}
}

func TestTimeoutIsRetriedThenReported(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer srv.Close()
	cmc := mustNew(t, WithAPIKey("key"), WithBaseURL(srv.URL),
		WithTimeout(50*time.Millisecond), WithMaxRetries(1), recordDelays(new([]time.Duration)))

	_, err := cmc.GlobalMetrics.QuotesLatest(context.Background(), nil)
	var timeoutErr *APITimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("error = %v, want *APITimeoutError", err)
	}
	var connErr *APIConnectionError
	if !errors.As(err, &connErr) {
		t.Errorf("*APITimeoutError should also match *APIConnectionError")
	}
	if timeoutErr.Timeout != 50*time.Millisecond {
		t.Errorf("Timeout = %v", timeoutErr.Timeout)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("%d attempts, want 2", n)
	}
}

func TestSlowBodyWithinTimeoutIsRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":`)
		w.(http.Flusher).Flush()
		time.Sleep(100 * time.Millisecond)
		_, _ = io.WriteString(w, `{"error_code":0}}`)
	}))
	defer srv.Close()
	cmc := mustNew(t, WithAPIKey("key"), WithBaseURL(srv.URL), WithTimeout(2*time.Second))
	if _, err := cmc.GlobalMetrics.QuotesLatest(context.Background(), nil); err != nil {
		t.Fatalf("reading the body after the headers failed: %v", err)
	}
}

func TestCallerCancellationIsNotRetried(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		cancel()
		<-r.Context().Done()
	}))
	defer srv.Close()
	cmc := mustNew(t, WithAPIKey("key"), WithBaseURL(srv.URL), recordDelays(new([]time.Duration)))

	_, err := cmc.GlobalMetrics.QuotesLatest(ctx, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	var connErr *APIConnectionError
	if errors.As(err, &connErr) {
		t.Errorf("cancellation should not be an *APIConnectionError: %v", err)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("%d attempts, want 1", n)
	}
}

func TestCancellationDuringBackoffStops(t *testing.T) {
	srv := newTestServer(t, reply{status: 503, body: `{}`})
	ctx, cancel := context.WithCancel(context.Background())
	stopDuringSleep := func(c *config) {
		c.sleep = func(ctx context.Context, _ time.Duration) error {
			cancel()
			<-ctx.Done()
			return ctx.Err()
		}
	}
	cmc := mustNew(t, WithAPIKey("key"), WithBaseURL(srv.URL), stopDuringSleep)
	if _, err := cmc.GlobalMetrics.QuotesLatest(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if n := len(srv.Requests()); n != 1 {
		t.Errorf("%d requests, want 1", n)
	}
}

func TestNetworkErrorsAreRetried(t *testing.T) {
	var calls atomic.Int32
	failing := doerFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("connection refused")
	})
	cmc := mustNew(t, WithAPIKey("key"), WithHTTPClient(failing), recordDelays(new([]time.Duration)))

	_, err := cmc.GlobalMetrics.QuotesLatest(context.Background(), nil)
	var connErr *APIConnectionError
	if !errors.As(err, &connErr) || !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("error = %v, want *APIConnectionError", err)
	}
	var timeoutErr *APITimeoutError
	if errors.As(err, &timeoutErr) {
		t.Errorf("a refused connection is not a timeout")
	}
	if n := calls.Load(); n != 3 {
		t.Errorf("%d attempts, want 3", n)
	}
}

func TestCustomHTTPClientIsUsed(t *testing.T) {
	var seen string
	custom := doerFunc(func(req *http.Request) (*http.Response, error) {
		seen = req.URL.String()
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
		}, nil
	})
	cmc := mustNew(t, WithAPIKey("key"), WithHTTPClient(custom))
	if _, err := cmc.GlobalMetrics.QuotesLatest(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if seen != ProBaseURL+"/v1/global-metrics/quotes/latest" {
		t.Errorf("URL = %s", seen)
	}
}
