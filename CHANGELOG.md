# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added

- `ErrRedirectNotAllowed` sentinel returned (wrapped) when a token
  endpoint responds with a redirect. Callers can now detect this
  specific security condition via `errors.Is` rather than string-
  matching the message.

### Security

- Default `*http.Client` used by `ExchangeCode`/`Refresh` now refuses
  to follow redirects on the token endpoint POST. A 30x response would
  otherwise re-POST the body (which carries `client_secret`,
  `refresh_token`, `code_verifier`) to the redirect target. Token
  endpoints do not redirect in practice; we fail closed.
- Token endpoint response bodies are now read through a 1 MiB
  `io.LimitReader` cap. A hostile or misconfigured server can no
  longer force unbounded buffering.
- `expires_in` from the token response is clamped to ~68 years before
  the `time.Duration` math runs. A hostile server returning e.g.
  `1e18` previously overflowed the multiplication and produced a
  past-dated `ExpiresAt`, making `Token.Expired()` return true
  immediately and potentially driving a refresh loop.
- Caller-supplied `Content-Type` in `Headers` is now dropped — it
  belongs to the `BodyEncoder` and was previously *added* alongside
  the encoder's value, yielding two conflicting headers.
- Token-endpoint URL parse failures now surface as `*TokenError`
  rather than panicking, so callers piping config/env-supplied values
  into `TokenURL` get a normal error path.
- `TokenError.Body` doc now warns that on a malformed 2xx response it
  may carry `access_token`/`refresh_token` material and must not be
  logged verbatim. `TokenError.Error()` itself never prints `Body`.

### Added

- **Stateless token primitives** for the standard token endpoint dance
  (RFC 6749 §4.1.3 + §6), so callers can collapse the boilerplate
  around `grant_type=authorization_code` and `grant_type=refresh_token`
  back into a few lines:
    - `Token` — parsed token response: `AccessToken`, `TokenType`,
      `RefreshToken`, `ExpiresAt` (computed from `expires_in` at receive
      time), `Scope`, plus `Raw map[string]any` preserving every
      top-level field for provider-specific extraction (id_token,
      account_id, api_key, …). Helpers `Expired()` and
      `ExpiresWithin(d)`.
    - `ExchangeCode(ctx, ExchangeParams) (*Token, error)` — POSTs
      `authorization_code` to a token endpoint, parses JSON, returns
      `Token`.
    - `Refresh(ctx, RefreshParams) (*Token, error)` — same shape for
      `refresh_token`.
    - `BodyEncoder` hook — defaults to RFC-standard form encoding.
      Pass `JSONBodyEncoder` for the small set of providers (Anthropic)
      that send JSON to the token endpoint.
    - `TokenError` — typed error matching RFC 6749 §5.2
      (`Code`, `Description`, `URI`, `HTTPStatus`, `Body`, `Err`),
      reachable via `errors.As`. Wraps transport errors via `Unwrap`.
- **Honest non-goals**: no `TokenSource` / auto-refreshing
  `http.Client` / background goroutines / token storage. The caller
  decides when to refresh and where to persist. See `doc.go`.
- `AwaitAuthCode` — orchestrates the "loopback callback OR manual paste,
  whichever wins" race. Composes `StartCallbackServer`'s result channel
  with an optional manual-input function (parsed via
  `ParseAuthorizationInput`), respects `ctx`, and dismisses the loser's
  visible prompt via an optional callback. Returns `ErrCallbackClosed`
  (sentinel) when the callback channel closes without delivering.
- `ExampleStartCallbackServer` — testable example exercising the headline
  API (visible on pkg.go.dev).
- Sync test asserting every placeholder constant appears in the embedded
  `callback_page.html`.

### Changed

- Replaced the legacy `unreachable.go` convention with a per-feature
  `<feature>_must.go` model. Each `mustX` helper now sits next to the
  code that uses it: `pkce_must.go` (`mustReadRandom`) and
  `callback_server_must.go` (`mustServeLoopback`). Helpers panic on
  structurally-unreachable error paths so callers have no impossible
  branch left to cover. `.covignore` now excludes `_must\.go:` instead
  of `/unreachable\.go:`. The `Makefile` coverage rule strips comments
  and blank lines from `.covignore` before feeding it to `grep -E`.
- **Breaking:** `Provider.Login` now takes `(ctx context.Context,
  callbacks LoginCallbacks)` instead of `(callbacks LoginCallbacks)`,
  and `Provider.RefreshToken` now takes `(ctx, creds)` instead of just
  `(creds)`. Context belongs as a method argument, not buried inside
  the callbacks struct.
- **Breaking:** removed `LoginCallbacks.Ctx`. Cancellation is now
  conveyed exclusively via the `ctx` argument to `Provider.Login`.
- **Breaking:** renamed `StartOAuthCallbackServer` to `StartCallbackServer`
  to remove package-name stutter (`pinoauth.StartCallbackServer`).
- Errors returned from `GeneratePKCE` and `StartCallbackServer` are now
  prefixed with `pinoauth:` and wrap the underlying cause with `%w`.
- Improved doc comments across the public API: every exported symbol
  and field now carries a doc comment, and `StartCallbackServer`
  documents its goroutine / lifecycle / channel-close contract.
- `make check` now runs `staticcheck ./...` when the tool is on `PATH`
  (skipped otherwise — staticcheck is not a hard build dep).

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
