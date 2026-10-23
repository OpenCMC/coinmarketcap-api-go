package coinmarketcap

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// CMCError is an HTTP error response from the API (status 400 or higher). The
// typed errors below wrap it, so errors.As finds both:
//
//	var rateLimited *coinmarketcap.RateLimitError
//	if errors.As(err, &rateLimited) {
//		retryAfter := rateLimited.Header.Get("Retry-After")
//	}
type CMCError struct {
	// StatusCode is the HTTP status code.
	StatusCode int
	// ErrorCode is status.error_code from the response body, or 0.
	ErrorCode int
	// Message is status.error_message from the response body, if any.
	Message string
	// Header holds the response headers, e.g. Retry-After on a 429.
	Header http.Header
	// Body is the raw response body.
	Body []byte
}

func (e *CMCError) Error() string {
	msg := fmt.Sprintf("coinmarketcap: request failed with status %d", e.StatusCode)
	if e.Message != "" {
		msg += ": " + e.Message
	}
	if e.ErrorCode != 0 {
		msg += fmt.Sprintf(" (error_code %d)", e.ErrorCode)
	}
	return msg
}

// BadRequestError is a 400 response: a parameter is missing or invalid.
type BadRequestError struct{ *CMCError }

// AuthenticationError is a 401 response: the API key is missing or invalid.
type AuthenticationError struct{ *CMCError }

// PaymentRequiredError is a 402 response: the key's plan needs activating,
// has expired or has an overdue balance.
type PaymentRequiredError struct{ *CMCError }

// ForbiddenError is a 403 response: the key's plan doesn't include the
// endpoint (error 1006), or the key is missing or disabled.
type ForbiddenError struct{ *CMCError }

// NotFoundError is a 404 response.
type NotFoundError struct{ *CMCError }

// RateLimitError is a 429 response: too many requests. Header carries
// Retry-After when the API sends it.
type RateLimitError struct{ *CMCError }

// InternalServerError is a 5xx response.
type InternalServerError struct{ *CMCError }

func (e *BadRequestError) Unwrap() error      { return e.CMCError }
func (e *AuthenticationError) Unwrap() error  { return e.CMCError }
func (e *PaymentRequiredError) Unwrap() error { return e.CMCError }
func (e *ForbiddenError) Unwrap() error       { return e.CMCError }
func (e *NotFoundError) Unwrap() error        { return e.CMCError }
func (e *RateLimitError) Unwrap() error       { return e.CMCError }
func (e *InternalServerError) Unwrap() error  { return e.CMCError }

// APIConnectionError means no response arrived: DNS, connection refused or
// reset, TLS and similar failures, after retries.
type APIConnectionError struct {
	Err error
}

func (e *APIConnectionError) Error() string {
	return "coinmarketcap: connection error: " + e.Err.Error()
}
func (e *APIConnectionError) Unwrap() error { return e.Err }

// APITimeoutError means every attempt took longer than the client's timeout.
// It wraps an *APIConnectionError, so errors.As finds either.
type APITimeoutError struct {
	Timeout time.Duration
	conn    *APIConnectionError
}

func (e *APITimeoutError) Error() string {
	return fmt.Sprintf("coinmarketcap: request timed out after %s", e.Timeout)
}
func (e *APITimeoutError) Unwrap() error { return e.conn }

// DecodeError means a successful response didn't match the SDK's types,
// usually because the API returned a field in a different shape than the
// OpenAPI spec says. The method still returns everything that did decode,
// and Body has the raw response.
type DecodeError struct {
	StatusCode int
	Body       []byte
	Err        error
}

func (e *DecodeError) Error() string { return "coinmarketcap: decoding response: " + e.Err.Error() }
func (e *DecodeError) Unwrap() error { return e.Err }

// newAPIError builds the typed error for an HTTP error response.
func newAPIError(status int, header http.Header, body []byte) error {
	base := &CMCError{StatusCode: status, Header: header, Body: body}
	base.ErrorCode, base.Message = parseErrorBody(body)
	switch {
	case status == http.StatusBadRequest:
		return &BadRequestError{base}
	case status == http.StatusUnauthorized:
		return &AuthenticationError{base}
	case status == http.StatusPaymentRequired:
		return &PaymentRequiredError{base}
	case status == http.StatusForbidden:
		return &ForbiddenError{base}
	case status == http.StatusNotFound:
		return &NotFoundError{base}
	case status == http.StatusTooManyRequests:
		return &RateLimitError{base}
	case status >= 500:
		return &InternalServerError{base}
	}
	return base
}

// parseErrorBody reads status.error_code (a number or a numeric string) and
// status.error_message, falling back to a top-level message or error field.
func parseErrorBody(body []byte) (int, string) {
	var parsed struct {
		Status *struct {
			ErrorCode    json.RawMessage `json:"error_code"`
			ErrorMessage *string         `json:"error_message"`
		} `json:"status"`
		Message *string `json:"message"`
		Error   *string `json:"error"`
	}
	if json.Unmarshal(body, &parsed) != nil {
		return 0, ""
	}
	code, msg := 0, ""
	if parsed.Status != nil {
		raw := string(parsed.Status.ErrorCode)
		if unquoted, err := strconv.Unquote(raw); err == nil {
			raw = unquoted
		}
		code, _ = strconv.Atoi(raw)
		if parsed.Status.ErrorMessage != nil {
			msg = *parsed.Status.ErrorMessage
		}
	}
	switch {
	case msg != "":
	case parsed.Message != nil:
		msg = *parsed.Message
	case parsed.Error != nil:
		msg = *parsed.Error
	}
	return code, msg
}
