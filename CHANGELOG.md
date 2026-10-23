# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
The Go, TypeScript and Python SDKs share one version number (see Versioning in the README).

## [5.0.0] - Unreleased

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
