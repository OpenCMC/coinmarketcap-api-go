package coinmarketcap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// decode turns a generated client call into the typed 200 body or an error.
func decode[T any](resp *http.Response, err error) (*T, error) {
	body, err := readBody(resp, err)
	if err != nil {
		return nil, err
	}
	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		decodeErr := &DecodeError{StatusCode: resp.StatusCode, Body: body, Err: err}
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			// encoding/json fills every field it can before reporting the
			// first mismatch, so the partial value is still useful.
			return &out, decodeErr
		}
		return nil, decodeErr
	}
	return &out, nil
}

// decodeRaw is decode for endpoints whose spec has no JSON 200 schema.
func decodeRaw(resp *http.Response, err error) (json.RawMessage, error) {
	body, err := readBody(resp, err)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(body), nil
}

func readBody(resp *http.Response, err error) ([]byte, error) {
	if err != nil {
		return nil, requestError(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, requestError(&APIConnectionError{Err: err})
	}
	if resp.StatusCode >= 400 {
		return nil, newAPIError(resp.StatusCode, resp.Header, body)
	}
	return body, nil
}

// requestError passes through the SDK's typed errors and the caller's
// context errors. Anything else failed while building the request.
func requestError(err error) error {
	var connErr *APIConnectionError
	if errors.As(err, &connErr) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("coinmarketcap: %w", err)
}
