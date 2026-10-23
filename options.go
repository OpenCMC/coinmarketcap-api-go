package coinmarketcap

import (
	"context"
	"net/http"
	"time"
)

// Defaults used by New.
const (
	DefaultTimeout    = 30 * time.Second
	DefaultMaxRetries = 2
)

// Option configures a Client.
type Option func(*config)

// HTTPDoer sends HTTP requests. *http.Client implements it.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type config struct {
	apiKey      string
	environment Environment
	baseURL     string
	httpClient  HTTPDoer
	timeout     time.Duration
	maxRetries  int

	// Overridden in tests.
	sleep  func(ctx context.Context, d time.Duration) error
	random func() float64
}

// WithAPIKey sets the Pro API key, sent in the X-CMC_PRO_API_KEY header. Get a
// free key at https://coinmarketcap.com/api/. The key is not sent in the public
// environment.
func WithAPIKey(key string) Option {
	return func(c *config) { c.apiKey = key }
}

// WithEnvironment selects the API: EnvironmentPro (the default) or the keyless
// EnvironmentPublic.
func WithEnvironment(env Environment) Option {
	return func(c *config) { c.environment = env }
}

// WithBaseURL replaces the environment's base URL.
func WithBaseURL(baseURL string) Option {
	return func(c *config) { c.baseURL = baseURL }
}

// WithHTTPClient sets the client that sends requests. The default is
// http.DefaultClient.
func WithHTTPClient(client HTTPDoer) Option {
	return func(c *config) { c.httpClient = client }
}

// WithTimeout sets the time limit for each attempt, including reading the
// response body. The default is 30 seconds. Zero or less means no limit.
func WithTimeout(d time.Duration) Option {
	return func(c *config) { c.timeout = d }
}

// WithMaxRetries sets how many times a failed request is retried. The default
// is 2. Zero disables retries.
func WithMaxRetries(n int) Option {
	return func(c *config) { c.maxRetries = n }
}
