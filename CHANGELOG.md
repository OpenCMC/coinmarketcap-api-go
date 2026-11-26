# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
The Go, TypeScript and Python SDKs share one version number (see Versioning in the README).

## [6.0.0] - 2026-11-20

Rebuilt from the OpenAPI spec published at
https://coinmarketcap.com/api/documentation/openapi.json (spec version `3.0.3`,
sha256 `a335f0d5…49da`).

### Added

- `cmc.Cryptocurrency.OhlcvHistorical` now calls `GET /v5/cryptocurrency/ohlcv/historical`
  (see Changed). Each candle is a row of 10 values: open, high, low, close, volume,
  market cap, then the open, high, low and close times.
- `cmc.RealWorldAssets.QuotesHistorical`: `GET /v5/real-world-assets/quotes/historical`,
  historical quotes for tokenized real-world assets.
- `Status` has `Category` and `ErrorDetail`, which some v5 endpoints return with an error.

### Changed (breaking)

- The module path is `github.com/OpenCMC/coinmarketcap-api-go/v6`. Update your imports:
  `go get github.com/OpenCMC/coinmarketcap-api-go/v6`.
- `cmc.Cryptocurrency.OhlcvHistorical` calls v5 instead of the deprecated v2 endpoint.
  `CryptocurrencyOhlcvHistoricalParams` has `ID` or `Slug`, a required `Interval`
  (`15m`, `1h`, `4h`, `1d`, `1w`, `1M`), `TimeStart`, `TimeEnd`, `Count` and `ConvertID`,
  and the method returns `*CryptocurrencyOHLCVHistoricalV5ResponseModel`. To keep calling v2,
  use `cmc.Deprecated.GetV2CryptocurrencyOhlcvHistorical`.
- RWA market pairs: `MarketPairQuotes` is `MarketPairQuote` (`market_pair_quote`, the name
  the API returns). Its types are renamed to match: `RWAMarketPairsResultsObject_MarketPairs_MarketPairQuote`
  and `RWAMarketPairsResultsObjectMarketPairsMarketPairQuoteCurrencyType` and its constants.
- `CryptocurrencyOhlcvHistoricalParams` is now the v5 params type; the v2 params type is
  `DeprecatedGetV2CryptocurrencyOhlcvHistoricalParams`.
- `cmc.Cryptocurrency.QuotesHistorical` (v3), `cmc.Deprecated.CryptocurrencyQuotesHistorical` (v1)
  and `cmc.Deprecated.GetV2CryptocurrencyQuotesHistorical`: each value in the `data` map is one
  object, or an array of objects when you request by `symbol` and several assets match. The map
  value is now a union; read it with `AsCryptocurrencyQuotesHistoricalResultObject()`:

  ```go
  m, err := res.Data.AsCryptocurrencyQuotesHistoricalResultsMap()
  // 5.x
  quote := m["1"]
  // 6.0.0
  quote, err := m["1"].AsCryptocurrencyQuotesHistoricalResultObject()
  ```

  The map values of `cmc.Deprecated.CryptocurrencyOhlcvHistorical` (v1) and
  `cmc.Deprecated.GetV2CryptocurrencyOhlcvHistorical` work the same way
  (`AsCryptocurrencyOHLCVHistoricalResultsObject()`).
- `Status.ErrorCode` is an `interface{}`: v5 endpoints send it as a string, older ones as
  a number.

## [5.0.1] - 2026-10-30

Rebuilt from the OpenAPI spec published at
https://coinmarketcap.com/api/documentation/openapi.json (spec version `3.0.3`,
sha256 `3e72a1c4…17c9`).

### Fixed

- `cmc.CmcIndex.IndexCMC100Latest` and `cmc.CmcIndex.IndexCMC20Latest` no longer return a
  `*DecodeError`. `LastUpdate` on `IndexLatestDTO` and `CMC20IndexLatestDTO` is a
  `*time.Time` now; it was an `*openapi_types.Date`, which can't hold the timestamp the
  API returns. This fixes the known issue in 5.0.0.

### Changed

- Doc comments follow the updated endpoint descriptions, for example which API plans
  include each endpoint.

## [5.0.0] - 2026-10-23

First release of the Go SDK. It is generated from the OpenAPI spec published at
https://coinmarketcap.com/api/documentation/openapi.json (spec version `3.0.3`,
sha256 `855b21bb…3e0b`), and starts at 5.0.0 to share its version with the TypeScript
and Python SDKs.

### Added

- `coinmarketcap.New` with API key authentication, the pro and keyless public
  environments, retries with backoff, per-attempt timeouts and typed errors.
- 110 endpoint methods in 16 categories, for example `cmc.Cryptocurrency.QuotesLatest`,
  each with a params type named after it (`CryptocurrencyQuotesLatestParams`).
- Every model, params and request body type from the spec, re-exported from the
  `coinmarketcap` package.
- `cmc.Raw()`, the generated low-level client, for access to the HTTP response.

### Known issues

- `cmc.CmcIndex.IndexCMC100Latest` and `cmc.CmcIndex.IndexCMC20Latest` return a
  `*DecodeError` along with the decoded response: the spec types `data.last_update` as
  a date, but the API returns a timestamp.
