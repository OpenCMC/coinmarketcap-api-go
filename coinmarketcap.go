// Package coinmarketcap is the official Go SDK for the CoinMarketCap API.
//
// Endpoints are grouped by category on the client. Every method takes a
// context and returns the typed response:
//
//	cmc, err := coinmarketcap.New(coinmarketcap.WithAPIKey(os.Getenv("CMC_PRO_API_KEY")))
//	if err != nil {
//		log.Fatal(err)
//	}
//	quotes, err := cmc.Cryptocurrency.QuotesLatest(ctx, &coinmarketcap.CryptocurrencyQuotesLatestParams{
//		ID: coinmarketcap.String("1,1027"),
//	})
//
// The keyless public API needs no key and serves a subset of the endpoints:
//
//	cmc, err := coinmarketcap.New(coinmarketcap.WithEnvironment(coinmarketcap.EnvironmentPublic))
//
// Failed requests are retried on 408, 409, 429 and 5xx responses, on network
// errors and on timeouts. Each attempt times out after 30 seconds. HTTP error
// responses come back as typed errors such as *RateLimitError; use errors.As
// to inspect them.
package coinmarketcap

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/OpenCMC/coinmarketcap-api-go/v5/internal/generated"
)

// Environment selects which API the client calls.
type Environment string

const (
	// EnvironmentPro is the Pro API. It needs an API key.
	EnvironmentPro Environment = "pro"
	// EnvironmentPublic is the keyless public API. It needs no key and serves
	// a subset of the endpoints.
	EnvironmentPublic Environment = "public"
)

// Base URLs of the environments.
const (
	ProBaseURL    = "https://pro-api.coinmarketcap.com"
	PublicBaseURL = "https://pro-api.coinmarketcap.com/public-api"
)

// ErrMissingAPIKey is returned by New when the pro environment has no API key.
var ErrMissingAPIKey = errors.New("coinmarketcap: an API key is required for the pro environment; " +
	"pass WithAPIKey, or WithEnvironment(EnvironmentPublic) for keyless access")

// Client calls the CoinMarketCap API. Create one with New and reuse it. It is
// safe for concurrent use.
type Client struct {
	// Services holds the endpoints by category, e.g. cmc.Cryptocurrency.
	Services

	raw         *generated.Client
	rawWithResp *generated.ClientWithResponses
	baseURL     string
	environment Environment
	apiKey      string
}

// New creates a client. The pro environment (the default) needs WithAPIKey.
func New(opts ...Option) (*Client, error) {
	cfg := config{
		environment: EnvironmentPro,
		timeout:     DefaultTimeout,
		maxRetries:  DefaultMaxRetries,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	baseURL := cfg.baseURL
	switch cfg.environment {
	case EnvironmentPro:
		if cfg.apiKey == "" {
			return nil, ErrMissingAPIKey
		}
		if baseURL == "" {
			baseURL = ProBaseURL
		}
	case EnvironmentPublic:
		if baseURL == "" {
			baseURL = PublicBaseURL
		}
	default:
		return nil, fmt.Errorf("coinmarketcap: unknown environment %q", cfg.environment)
	}

	httpClient := cfg.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	doer := &retryingDoer{
		client:     httpClient,
		maxRetries: max(cfg.maxRetries, 0),
		timeout:    cfg.timeout,
		sleep:      cfg.sleep,
		random:     cfg.random,
	}

	c := &Client{baseURL: baseURL, environment: cfg.environment, apiKey: cfg.apiKey}
	raw, err := generated.NewClient(baseURL,
		generated.WithHTTPClient(doer),
		generated.WithRequestEditorFn(c.editRequest),
	)
	if err != nil {
		return nil, fmt.Errorf("coinmarketcap: %w", err)
	}
	c.raw = raw
	c.rawWithResp = &generated.ClientWithResponses{ClientInterface: raw}
	c.Services = newServices(c)
	return c, nil
}

// BaseURL returns the URL the client sends requests to.
func (c *Client) BaseURL() string { return c.baseURL }

// Environment returns the environment the client was created for.
func (c *Client) Environment() Environment { return c.environment }

// Raw returns the generated low-level client, which shares this client's
// authentication, retries and timeout. Its methods return the HTTP response
// together with the decoded body for each status code.
func (c *Client) Raw() *RawClient { return c.rawWithResp }

// RequestEditorFn changes a request before it is sent, for example to add a
// header. Every method accepts any number of them.
type RequestEditorFn = generated.RequestEditorFn

func (c *Client) editRequest(_ context.Context, req *http.Request) error {
	if c.environment == EnvironmentPro && c.apiKey != "" {
		req.Header.Set("X-CMC_PRO_API_KEY", c.apiKey)
	}
	req.Header.Set("Accept", "application/json")
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "coinmarketcap-sdk-go/"+Version)
	}
	return nil
}
