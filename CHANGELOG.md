# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Changed

- **Breaking:** renamed `StartOAuthCallbackServer` to `StartCallbackServer`
  to remove package-name stutter (`pinoauth.StartCallbackServer`).
- Errors returned from `GeneratePKCE` and `StartCallbackServer` are now
  prefixed with `pinoauth:` and wrap the underlying cause with `%w`.
- Improved doc comments across the public API: every exported symbol
  and field now carries a doc comment, and `StartCallbackServer`
  documents its goroutine / lifecycle / channel-close contract.
- `make check` now runs `staticcheck ./...` when the tool is on `PATH`
  (skipped otherwise — staticcheck is not a hard build dep).

### Added

- `ExampleStartCallbackServer` — testable example exercising the headline
  API (visible on pkg.go.dev).
- Sync test asserting every placeholder constant appears in the embedded
  `callback_page.html`.

### Removed

- Unexported `oauthHTTPClient` package-level var (dead code in pinoauth —
  it was a vestige of fir's concrete provider implementations and had no
  consumers in the toolkit).

### Earlier in this cycle

- `make all` (and `make test`) now enforce a 100% coverage gate via
  `.covignore`, mirroring the mechanism used in sibling repos
  (firpty, skipstone). Adds `gofmt -l` + `go vet` checks. New targets:
  `check`, `fmt`, `fmtcheck`, `test-fast`, `open_coverage`.

## [0.1.0] - 2026-05-06

### Added
- Initial release. Extracted from [fir](https://github.com/kfet/fir)'s
  `pkg/ai/oauth` package.
- `GeneratePKCE` — RFC 7636 PKCE verifier + S256 challenge.
- `StartCallbackServer` — loopback HTTP callback server with state
  validation, embedded styled success/error page, and graceful shutdown.
- `ParseAuthorizationInput` — parse pasted auth codes from full callback
  URLs, `code#state` form, query-string fragments, or bare codes.
- `Provider`, `Credentials`, `LoginCallbacks`, `AuthInfo`, `Prompt`,
  `ProviderInfo` — shared types for building provider-specific flows.
- Stdlib-only. No third-party dependencies.
