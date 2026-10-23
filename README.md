# coinmarketcap-api-go

[![Go Reference](https://pkg.go.dev/badge/github.com/OpenCMC/coinmarketcap-api-go/v5.svg)](https://pkg.go.dev/github.com/OpenCMC/coinmarketcap-api-go/v5)

The official Go SDK for the [CoinMarketCap Pro API](https://coinmarketcap.com/api/documentation/).

## Installation

```bash
go get github.com/OpenCMC/coinmarketcap-api-go/v5
```

The package name is `coinmarketcap`:

```go
import "github.com/OpenCMC/coinmarketcap-api-go/v5"
```

## Quick Start

No API key needed. The keyless public API serves prices, listings, global
metrics, the Fear and Greed Index and DEX data:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/OpenCMC/coinmarketcap-api-go/v5"
)

func main() {
	cmc, err := coinmarketcap.New(coinmarketcap.WithEnvironment(coinmarketcap.EnvironmentPublic))
	if err != nil {
		log.Fatal(err)
	}

	prices, err := cmc.Cryptocurrency.SimplePrice(context.Background(), &coinmarketcap.CryptocurrencySimplePriceParams{
		ID: coinmarketcap.String("1,1027"), // 1 = BTC, 1027 = ETH
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, coin := range prices.Data {
		for _, q := range coin.Quotes {
			fmt.Printf("%s: %.2f %s\n", coin.Name, q.Price, q.Symbol)
		}
	}
}
```

With a [free API key](https://coinmarketcap.com/api/) you get every endpoint
your plan includes, usage tracking and higher limits:

```go
cmc, err := coinmarketcap.New(coinmarketcap.WithAPIKey(os.Getenv("CMC_PRO_API_KEY")))
if err != nil {
	log.Fatal(err)
}

quotes, err := cmc.Cryptocurrency.QuotesLatest(ctx, &coinmarketcap.CryptocurrencyQuotesLatestParams{
	ID: coinmarketcap.String("1,1027"),
})
```

## Usage

### Endpoints by category

Endpoints are grouped by category on the client. Every method takes a
`context.Context`, then the endpoint's params (if it has any), and returns the
decoded response:

```go
cmc.Cryptocurrency.QuotesLatest(ctx, &coinmarketcap.CryptocurrencyQuotesLatestParams{ID: coinmarketcap.String("1")})
cmc.Cryptocurrency.ListingsLatest(ctx, &coinmarketcap.CryptocurrencyListingsLatestParams{Limit: coinmarketcap.String("10")})
cmc.GlobalMetrics.QuotesLatest(ctx, nil)
cmc.Exchange.Info(ctx, &coinmarketcap.ExchangeInfoParams{ID: coinmarketcap.String("270")})
```

Each method's params type is named after the category and method, for example
`CryptocurrencyQuotesLatestParams`. Pass `nil` when you don't need any params.
Optional params are pointers; `coinmarketcap.String`, `Int`, `Int64`,
`Float64`, `Bool` and the generic `Ptr` make them inline.

### POST endpoints

POST endpoints take a request body, named the same way with `Body`:

```go
tokens := []coinmarketcap.PlatformAddress{{
	Platform: coinmarketcap.String("ethereum"),
	Address:  coinmarketcap.String("0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"),
}}
prices, err := cmc.Token.BatchGetTokenPrice(ctx, coinmarketcap.TokenBatchGetTokenPriceBody{Tokens: &tokens})
```

### Configuration

```go
cmc, err := coinmarketcap.New(
	coinmarketcap.WithAPIKey("your-api-key"),         // required for the pro environment
	coinmarketcap.WithEnvironment(coinmarketcap.EnvironmentPro), // EnvironmentPro (default) or EnvironmentPublic
	coinmarketcap.WithBaseURL("https://..."),         // replace the environment's base URL
	coinmarketcap.WithHTTPClient(&http.Client{}),     // default: http.DefaultClient
	coinmarketcap.WithTimeout(10*time.Second),        // per attempt, default 30s
	coinmarketcap.WithMaxRetries(3),                  // default 2, 0 disables retries
)
```

The client is safe for concurrent use. Create one and reuse it.

### Request editors

Every method accepts `RequestEditorFn` values that change the request before it
is sent, for example to add a header:

```go
traceID := func(ctx context.Context, req *http.Request) error {
	req.Header.Set("X-Request-ID", "abc-123")
	return nil
}
cmc.Tools.KeyInfo(ctx, traceID)
```

### Low-level client

`cmc.Raw()` returns the generated client, which shares the SDK's
authentication, retries and timeout. Its methods return the HTTP response with
the body decoded for each status code:

```go
resp, err := cmc.Raw().GetV1KeyInfoWithResponse(ctx)
if err == nil && resp.JSON200 != nil {
	fmt.Println(resp.StatusCode(), resp.HTTPResponse.Header.Get("Content-Type"))
}
```

## Error Handling

HTTP errors come back as typed errors. Use `errors.As` to inspect them:

```go
quotes, err := cmc.Cryptocurrency.QuotesLatest(ctx, params)

var rateLimited *coinmarketcap.RateLimitError
var apiErr *coinmarketcap.CMCError
switch {
case errors.As(err, &rateLimited):
	fmt.Println("rate limited, retry after", rateLimited.Header.Get("Retry-After"))
case errors.As(err, &apiErr):
	// Any HTTP error: StatusCode, ErrorCode and Message from the response,
	// Header and the raw Body.
	fmt.Println(apiErr.StatusCode, apiErr.ErrorCode, apiErr.Message)
case err != nil:
	fmt.Println(err)
}
```

| Status Code | Error Type              |
| ----------- | ----------------------- |
| 400         | `*BadRequestError`      |
| 401         | `*AuthenticationError`  |
| 402         | `*PaymentRequiredError` |
| 403         | `*ForbiddenError`       |
| 404         | `*NotFoundError`        |
| 429         | `*RateLimitError`       |
| 5xx         | `*InternalServerError`  |
| Other 4xx   | `*CMCError`             |
| Network     | `*APIConnectionError`   |
| Timeout     | `*APITimeoutError`      |

Every HTTP error type wraps a `*CMCError`, and `*APITimeoutError` wraps an
`*APIConnectionError`, so `errors.As` finds the general type too.

If a successful response doesn't match the SDK's types, the method returns a
`*DecodeError` together with everything that did decode. `DecodeError.Body`
holds the raw response.

## Automatic Retries

Failed requests are retried with exponential backoff:

- **Retryable**: 408, 409, 429, 500, 502, 503, 504, network errors and timeouts
- **Default**: 2 retries with a 500ms initial delay, doubling each time, plus up to 20% jitter, up to 8s
- **429 responses**: Respects a numeric `Retry-After` header, capped at 8s so a large value can't stall the call
- **Body-safe**: Request bodies are sent again on each retry (POST endpoints included)

## Timeout and Cancellation

Each attempt times out after 30 seconds by default, including reading the
response body. A request that times out on every attempt returns
`*APITimeoutError`.

Cancel a request or set a deadline with the context. Cancellation is never
retried, and the method returns the context's error:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
quotes, err := cmc.Cryptocurrency.QuotesLatest(ctx, params) // errors.Is(err, context.DeadlineExceeded)
```

## Available Endpoints

All endpoints are methods on `cmc.<Category>`:

- `cmc.Cryptocurrency`: quotes, listings, market pairs, OHLCV, categories
- `cmc.Exchange`: exchange info, listings, market pairs
- `cmc.GlobalMetrics`: global stats, Fear and Greed Index, Altcoin Season Index
- `cmc.Derivatives`: derivatives markets and liquidations
- `cmc.RealWorldAssets`: tokenized real-world assets
- `cmc.Content` and `cmc.Community`: news, posts, trending tokens and topics
- `cmc.CmcIndex`: CMC 20 and CMC 100 indices
- `cmc.CmcAi`: CMC AI answers
- `cmc.Token`, `cmc.Platform`, `cmc.Holder`, `cmc.Ohlcv`: DEX tokens, pools, trades, holders and candles
- `cmc.Tools`: price conversion, fiat map, API key info

Methods marked keyless in their documentation also work in the public
environment. See the [API reference](https://coinmarketcap.com/api/documentation/)
for every endpoint and field.

## Versioning

The SDK follows [Semantic Versioning](https://semver.org). A major version removes or
renames methods, or changes return types or response fields in a way that can break
existing code. A minor version adds endpoints or fields. A patch version fixes bugs.

The Go, TypeScript (`@coinmarketcap/sdk`) and Python (`coinmarketcap-sdk`) SDKs are
released together with the same version number, so the module path ends in the major
version (`/v5`). The SDK version is separate from the API's endpoint versions (`/v1`,
`/v2`, `/v3`). Each [CHANGELOG](CHANGELOG.md) entry names the OpenAPI spec the release
was built from and lists its breaking changes.

## Requirements

- Go 1.24 or later

## License

MIT
