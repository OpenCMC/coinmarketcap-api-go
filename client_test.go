package coinmarketcap

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestNewRequiresAPIKeyForPro(t *testing.T) {
	if _, err := New(); !errors.Is(err, ErrMissingAPIKey) {
		t.Fatalf("New() error = %v, want ErrMissingAPIKey", err)
	}
	if _, err := New(WithEnvironment(EnvironmentPro)); !errors.Is(err, ErrMissingAPIKey) {
		t.Fatalf("New(pro) error = %v, want ErrMissingAPIKey", err)
	}
	if _, err := New(WithEnvironment(EnvironmentPublic)); err != nil {
		t.Fatalf("New(public) error = %v, want nil", err)
	}
	if _, err := New(WithAPIKey("key"), WithEnvironment("sandbox")); err == nil {
		t.Fatal("New(sandbox) succeeded, want an unknown environment error")
	}
}

func TestBaseURLs(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantURL string
		wantEnv Environment
	}{
		{"pro", []Option{WithAPIKey("key")}, ProBaseURL, EnvironmentPro},
		{"public", []Option{WithEnvironment(EnvironmentPublic)}, PublicBaseURL, EnvironmentPublic},
		{"override", []Option{WithAPIKey("key"), WithBaseURL("http://localhost:8080")}, "http://localhost:8080", EnvironmentPro},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := mustNew(t, tt.opts...)
			if c.BaseURL() != tt.wantURL || c.Environment() != tt.wantEnv {
				t.Errorf("got %s %s, want %s %s", c.BaseURL(), c.Environment(), tt.wantURL, tt.wantEnv)
			}
		})
	}
	if ProBaseURL != "https://pro-api.coinmarketcap.com" || PublicBaseURL != "https://pro-api.coinmarketcap.com/public-api" {
		t.Errorf("unexpected base URLs %s %s", ProBaseURL, PublicBaseURL)
	}
}

func TestProSendsAPIKey(t *testing.T) {
	srv := newTestServer(t)
	cmc := mustNew(t, WithAPIKey("test-key"), WithBaseURL(srv.URL))

	_, err := cmc.Cryptocurrency.QuotesLatest(context.Background(), &CryptocurrencyQuotesLatestParams{
		ID:      String("1,1027"),
		Convert: String("USD"),
	})
	if err != nil {
		t.Fatal(err)
	}
	req := srv.Requests()[0]
	if got := req.Header.Get("X-CMC_PRO_API_KEY"); got != "test-key" {
		t.Errorf("X-CMC_PRO_API_KEY = %q, want test-key", got)
	}
	if got := req.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q, want application/json", got)
	}
	if got := req.Header.Get("User-Agent"); got != "coinmarketcap-sdk-go/"+Version {
		t.Errorf("User-Agent = %q", got)
	}
	if req.Method != http.MethodGet || req.Path != "/v3/cryptocurrency/quotes/latest" {
		t.Errorf("request = %s %s", req.Method, req.Path)
	}
	query, _ := url.ParseQuery(req.Query)
	if query.Get("id") != "1,1027" || query.Get("convert") != "USD" {
		t.Errorf("query = %s", req.Query)
	}
}

func TestPublicOmitsAPIKeyAndKeepsBasePath(t *testing.T) {
	srv := newTestServer(t)
	cmc := mustNew(t,
		WithEnvironment(EnvironmentPublic),
		WithAPIKey("must-not-be-sent"),
		WithBaseURL(srv.URL+"/public-api"),
	)
	if _, err := cmc.GlobalMetrics.FearandgreedLatest(context.Background()); err != nil {
		t.Fatal(err)
	}
	req := srv.Requests()[0]
	if got := req.Header.Get("X-CMC_PRO_API_KEY"); got != "" {
		t.Errorf("X-CMC_PRO_API_KEY = %q, want none in the public environment", got)
	}
	if req.Path != "/public-api/v3/fear-and-greed/latest" {
		t.Errorf("path = %s, want /public-api/v3/fear-and-greed/latest", req.Path)
	}
}

func TestRequestEditors(t *testing.T) {
	srv := newTestServer(t)
	cmc := mustNew(t, WithAPIKey("key"), WithBaseURL(srv.URL))
	addHeader := func(_ context.Context, req *http.Request) error {
		req.Header.Set("X-Trace", "abc")
		return nil
	}
	if _, err := cmc.Tools.KeyInfo(context.Background(), addHeader); err != nil {
		t.Fatal(err)
	}
	if got := srv.Requests()[0].Header.Get("X-Trace"); got != "abc" {
		t.Errorf("X-Trace = %q, want abc", got)
	}

	failing := func(context.Context, *http.Request) error { return errors.New("editor failed") }
	_, err := cmc.Tools.KeyInfo(context.Background(), failing)
	if err == nil || !strings.Contains(err.Error(), "editor failed") {
		t.Fatalf("error = %v, want the editor's error", err)
	}
	var connErr *APIConnectionError
	if errors.As(err, &connErr) {
		t.Errorf("an editor error is not a connection error: %v", err)
	}
}

func TestRawClientSharesAuth(t *testing.T) {
	srv := newTestServer(t, reply{status: http.StatusOK, body: `{"status":{"error_code":0},"data":{}}`})
	cmc := mustNew(t, WithAPIKey("raw-key"), WithBaseURL(srv.URL))

	resp, err := cmc.Raw().GetV1KeyInfoWithResponse(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		t.Fatalf("status %d, JSON200 %v", resp.StatusCode(), resp.JSON200)
	}
	if got := srv.Requests()[0].Header.Get("X-CMC_PRO_API_KEY"); got != "raw-key" {
		t.Errorf("X-CMC_PRO_API_KEY = %q, want raw-key", got)
	}
}

func TestPOSTSendsJSONBody(t *testing.T) {
	srv := newTestServer(t)
	cmc := mustNew(t, WithAPIKey("key"), WithBaseURL(srv.URL))
	tokens := []PlatformAddress{{Platform: String("ethereum"), Address: String("0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48")}}

	if _, err := cmc.Token.BatchGetTokenPrice(context.Background(), TokenBatchGetTokenPriceBody{Tokens: &tokens}); err != nil {
		t.Fatal(err)
	}
	req := srv.Requests()[0]
	if req.Method != http.MethodPost || req.Path != "/v1/dex/token/price/batch" {
		t.Errorf("request = %s %s", req.Method, req.Path)
	}
	if !strings.HasPrefix(req.Header.Get("Content-Type"), "application/json") {
		t.Errorf("Content-Type = %q", req.Header.Get("Content-Type"))
	}
	want := `{"tokens":[{"address":"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48","platform":"ethereum"}]}`
	if req.Body != want {
		t.Errorf("body = %s, want %s", req.Body, want)
	}
}

func TestHelpers(t *testing.T) {
	if *String("a") != "a" || *Int(1) != 1 || *Int64(2) != 2 || *Float64(1.5) != 1.5 || !*Bool(true) || *Ptr(3) != 3 {
		t.Error("pointer helpers returned the wrong value")
	}
}
