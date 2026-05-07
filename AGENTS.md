# AGENTS.md

Guidance for AI agents working on `pinoauth`.

## Scope

`pinoauth` is a **small, focused** stdlib-only toolkit for browser-loopback
OAuth (RFC 8252) in CLI/native apps. Three building blocks:

- PKCE (`pkce.go`)
- Loopback callback server (`callback_server.go`, `callback_page.html`)
- Pasted-code parser (`parse_auth.go`)

Plus shared types (`types.go`) — including the `Provider` interface, which
is a *convention* for callers to follow, not something `pinoauth` itself
implements concretely.

**Do not** add concrete provider implementations (Anthropic, GitHub, etc.)
to this repo. They belong in the consumer's codebase.

## Constraints

- **Stdlib only.** No third-party deps. Ever. If you reach for one, stop
  and ask first.
- **Go 1.21+.** Don't use language features newer than that without a real
  need; bumping the minimum cuts users.
- **No global state.** No `init()` registries. No package-level mutables
  beyond the existing `oauthHTTPClient` (which exists so tests can swap it).
- **Tests use real HTTP**, not mocks — `httptest.Server` is fine.

## Workflow

- `make all` runs vet + race tests. Must pass before any commit.
- Add a `## [Unreleased]` entry in `CHANGELOG.md` for any user-visible
  change.
- Update `doc.go` and `README.md` when the public API changes.

## Public API

Anything exported from the package is API. Treat it as semver-stable from
v0.1.0 onward. Renames or signature changes need a major bump.
