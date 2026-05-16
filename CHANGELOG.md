# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

## [0.3.0] - 2026-05-16

### Changed

- **Breaking:** `ExchangeRequest.Extra` and `RefreshRequest.Extra` no
  longer silently overwrite the form fields pinoauth owns. Passing any
  of `grant_type`, `client_id`, `client_secret`, `code`, `code_verifier`,
  `redirect_uri` (Exchange) or `grant_type`, `client_id`,
  `client_secret`, `refresh_token`, `scope` (Refresh) via `Extra` now
  returns a validation error naming the offending key, rather than
  letting untrusted input override security-critical fields. Callers
  that legitimately need to vary those fields should set the dedicated
  `ExchangeRequest` / `RefreshRequest` / `Client` fields instead.

### Added

- `Extra` is now the supported way to inject non-standard token-body
  fields some providers require (e.g. Anthropic's `state` echo). Form
  and JSON (`JSONBodyEncoder`) bodies both carry the extra fields.

## [0.2.3] - 2026-05-12

### Added

- `AuthInfo.ShortURL` — optional pre-shortened form of `URL` produced
  by the provider (e.g. via a public URL shortener that forwards
  click-time query params). Callers typically present it prominently
  and fall back to `URL`. Empty means no short form. Zero-value
  backwards compatible with v0.2.2.

## [0.2.2] - 2026-05-12

### Fixed

- Retract `v0.2.0` and `v0.2.1`. Both were tagged-then-retagged during
  the initial public release, leaving the Go module proxy serving
  pre-fix content that doesn't match the corresponding git tags. The
  code in `v0.2.2` is equivalent to the (corrected) `v0.2.1` git tag —
  use `v0.2.2` or later.

## [0.2.1] - 2026-05-12

### Changed

- `ParseAuthorizationInput` no longer unconditionally strips backslashes.
  It now detects shell-escape "tells" (a backslash followed by `?`, `&`,
  `=`, `#`, or space) and only then performs a proper shell-unescape
  (`\\` → `\`, `\X` → `X`). Inputs without those tells are left alone,
  preserving backslashes that RFC 6749 §A.11 permits inside VSCHAR
  authorization codes.

## [0.2.0] - 2026-05-09

### Changed (breaking)

- **Token-endpoint API moved onto a `Client` struct.** The standalone
  `ExchangeCode` and `Refresh` functions are gone, replaced by:
    - `Client` — holds the per-provider config that doesn't change
      between requests: `TokenURL`, `ClientID`, `ClientSecret`,
      `HTTPClient`, `BodyEncoder`, `Headers`.
    - `Client.Exchange(ctx, ExchangeRequest)` / `Client.Refresh(ctx,
      RefreshRequest)` — methods take a small per-call request struct
      with just the grant-specific fields (`Code/CodeVerifier/
      RedirectURI/Extra` for exchange; `RefreshToken/Scope/Extra` for
      refresh).
  Migration: collapse the previous `ExchangeParams`/`RefreshParams`
  literals into a `Client{}` (set once, reuse for refresh) and a small
  `ExchangeRequest{}`/`RefreshRequest{}` per call. The seven common
  config fields no longer have to be repeated on every refresh.
- **`Provider` slimmed.** Removed `GetAPIKey`, `ListModels`, and the
  `Credentials` round-trip. The interface now exposes just
  `ID/Name/Login/RefreshToken/UsesCallbackServer` and traffics in
  `*Token` directly. API-key extraction, model listing, and other
  provider-specific concerns belong on the concrete type, not the
  shared shape — pinoauth is an OAuth toolkit, not an LLM-provider
  framework.
- **Removed `Credentials`.** Use `*Token` (which already carries
  `AccessToken`, `RefreshToken`, `ExpiresAt`, and `Raw` for everything
  provider-specific). The two types were redundant; `Token` is the
  better-designed of the two and the only one the token primitives
  actually used.
- **`GeneratePKCE` no longer returns an error.** It panics only on
  kernel CSPRNG failure (consistent with the rest of the `must*`
  doctrine) — the previous `error` return was always nil. Callers
  should drop the `, err :=` and any nil check.

### Added

- `GenerateState()` — 32-byte random base64url value for the OAuth
  `state` parameter (RFC 6749 §10.12). Closes the "bring your own RNG"
  footgun in the README quick-start.
- GitHub Actions CI workflow that runs `make all` across Go
  1.21/1.22/1.23 on linux/macos. `make all` is now the single
  source of truth for the strict pass: race detector, shuffled order,
  fresh test cache, gofmt, go vet, staticcheck, and the 100% coverage
  gate. The previously separate `make test` target is gone — there is
  no "fast" mode in the Makefile any more (run `go test ./...` directly
  if you want to iterate).

### Security

- `Client.HTTPClient` doc now warns that a caller-supplied client must
  configure `CheckRedirect` to refuse — Go's default follows up to 10
  redirects on POST, which would re-send the token-request body
  (carrying `client_secret`/`refresh_token`/`code_verifier`) to the
  redirect target. The bundled default client already refuses; this
  closes the documentation gap for the override path.

### Added (earlier in this cycle)

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
