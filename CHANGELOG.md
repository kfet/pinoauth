# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

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
