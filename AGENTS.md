# AGENTS.md

Guidance for AI agents working on `pinoauth`.

## Scope

`pinoauth` is a **small, focused** stdlib-only toolkit for browser-loopback
OAuth (RFC 8252) in CLI/native apps. Building blocks:

- PKCE — `GeneratePKCE` (`pkce.go`)
- State — `GenerateState` (`pkce.go`)
- Loopback callback server — `StartCallbackServer` (`callback_server.go`,
  `callback_page.html`)
- Pasted-code parser — `ParseAuthorizationInput` (`parse_auth.go`)
- Callback / manual-paste race — `AwaitAuthCode` (`await.go`)
- Token-endpoint primitives — `Client` (`TokenURL`, `ClientID`, …) with
  `Exchange` / `Refresh` methods, plus `ExchangeRequest`, `RefreshRequest`,
  `Token`, `*TokenError`, `TokenClient` interface (`token.go`)
- `Must*` panic-on-error wrappers (`*_must.go`)

Plus shared types (`types.go`) — including the `Provider` interface, which
is a *convention* for callers to follow, not something `pinoauth` itself
implements concretely.

`doc.go` is the source of truth for the public API surface; keep it and
this list in sync.

**Do not** add concrete provider implementations (Anthropic, GitHub, etc.)
to this repo. They belong in the consumer's codebase.

## Constraints

- **Stdlib only.** No third-party deps. Ever. If you reach for one, stop
  and ask first.
- **Go 1.21+.** Don't use language features newer than that without a real
  need; bumping the minimum cuts users.
- **No global state.** No `init()` registries. No package-level mutables.
- **Tests use real HTTP**, not mocks — `httptest.Server` is fine.

## Workflow

- `make all` runs vet + race tests. Must pass before any commit.
- Add a `## [Unreleased]` entry in `CHANGELOG.md` for any user-visible
  change.
- Update `doc.go`, `README.md`, and `AGENTS.md` when the public API changes.

## Public API

Anything exported from the package is API. Treat it as semver-stable from
v0.1.0 onward. Renames or signature changes need a major bump.
