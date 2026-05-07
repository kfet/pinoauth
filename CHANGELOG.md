# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Changed
- `make all` (and `make test`) now enforce a 100% coverage gate via
  `.covignore`, mirroring the mechanism used in sibling repos
  (firpty, skipstone). Adds `gofmt -l` + `go vet` checks. New targets:
  `check`, `fmt`, `fmtcheck`, `test-fast`, `open_coverage`.

## [0.1.0] - 2026-05-06

### Added
- Initial release. Extracted from [fir](https://github.com/kfet/fir)'s
  `pkg/ai/oauth` package.
- `GeneratePKCE` — RFC 7636 PKCE verifier + S256 challenge.
- `StartOAuthCallbackServer` — loopback HTTP callback server with state
  validation, embedded styled success/error page, and graceful shutdown.
- `ParseAuthorizationInput` — parse pasted auth codes from full callback
  URLs, `code#state` form, query-string fragments, or bare codes.
- `Provider`, `Credentials`, `LoginCallbacks`, `AuthInfo`, `Prompt`,
  `ProviderInfo` — shared types for building provider-specific flows.
- Stdlib-only. No third-party dependencies.
