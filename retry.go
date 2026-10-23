package coinmarketcap

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Same policy as the TypeScript and Python SDKs.
var retryableStatuses = map[int]bool{408: true, 409: true, 429: true, 500: true, 502: true, 503: true, 504: true}

const (
	initialRetryDelay = 500 * time.Millisecond
	maxRetryDelay     = 8 * time.Second
)

// retryingDoer sends a request with a per-attempt timeout and retries
// retryable statuses, network errors and timeouts. The caller's context is
// never retried: once it is done, the error is returned as is.
type retryingDoer struct {
	client     HTTPDoer
	maxRetries int
	timeout    time.Duration
	sleep      func(ctx context.Context, d time.Duration) error
	random     func() float64
}

func (d *retryingDoer) Do(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	getBody, err := replayableBody(req)
	if err != nil {
		return nil, &APIConnectionError{Err: err}
	}

	for attempt := 0; ; attempt++ {
		attemptCtx, cancel := ctx, context.CancelFunc(func() {})
		if d.timeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, d.timeout)
		}
		attemptReq := req.Clone(attemptCtx)
		if getBody != nil {
			body, err := getBody()
			if err != nil {
				cancel()
				return nil, &APIConnectionError{Err: err}
			}
			attemptReq.Body, attemptReq.GetBody = body, getBody
		}

		resp, err := d.client.Do(attemptReq)
		if err == nil {
			if !retryableStatuses[resp.StatusCode] || attempt >= d.maxRetries {
				// Keep the attempt's context alive until the body is read.
				resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
				return resp, nil
			}
			delay := retryDelay(attempt, resp.StatusCode, resp.Header, d.rand())
			drainAndClose(resp.Body)
			cancel()
			if err := d.wait(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}

		timedOut := errors.Is(attemptCtx.Err(), context.DeadlineExceeded)
		cancel()
		if ctx.Err() != nil {
			return nil, err // the caller cancelled or its deadline passed
		}
		var lastErr error = &APIConnectionError{Err: err}
		if timedOut {
			lastErr = &APITimeoutError{Timeout: d.timeout, conn: &APIConnectionError{Err: err}}
		}
		if attempt >= d.maxRetries {
			return nil, lastErr
		}
		if err := d.wait(ctx, retryDelay(attempt, 0, nil, d.rand())); err != nil {
			return nil, err
		}
	}
}

func (d *retryingDoer) rand() float64 {
	if d.random != nil {
		return d.random()
	}
	return rand.Float64()
}

func (d *retryingDoer) wait(ctx context.Context, delay time.Duration) error {
	if d.sleep != nil {
		return d.sleep(ctx, delay)
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// retryDelay returns how long to wait before the next attempt. A 429's
// numeric Retry-After is honoured but capped, so "Retry-After: 3600" can't
// stall the call for an hour. Otherwise the delay doubles from 500ms per
// attempt, plus up to 20% jitter, capped at 8s. random is in [0, 1).
func retryDelay(attempt, status int, header http.Header, random float64) time.Duration {
	if status == http.StatusTooManyRequests && header != nil {
		seconds, err := strconv.ParseFloat(strings.TrimSpace(header.Get("Retry-After")), 64)
		if err == nil && !math.IsNaN(seconds) {
			return time.Duration(min(max(seconds, 0), maxRetryDelay.Seconds()) * float64(time.Second))
		}
	}
	base := float64(initialRetryDelay) * math.Pow(2, float64(attempt))
	return time.Duration(min(base+base*0.2*random, float64(maxRetryDelay)))
}

// replayableBody returns a function that yields a fresh copy of the request
// body for each attempt, or nil when there is no body.
func replayableBody(req *http.Request) (func() (io.ReadCloser, error), error) {
	if req.Body == nil || req.Body == http.NoBody {
		return nil, nil
	}
	if req.GetBody != nil {
		return req.GetBody, nil
	}
	data, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return nil, err
	}
	return func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }, nil
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnClose) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

// drainAndClose reads what is left of a body so the connection can be reused.
func drainAndClose(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 1<<16))
	_ = body.Close()
}
