// Package pinoauth is a small, stdlib-only toolkit for building OAuth 2.0
// browser-loopback flows (RFC 8252, "OAuth 2.0 for Native Apps") in CLI and
// desktop applications.
//
// It provides the building blocks every native-app PKCE flow needs and
// nothing else:
//
//   - [GeneratePKCE] — RFC 7636 code verifier + S256 challenge.
//   - [StartCallbackServer] — loopback HTTP server that catches the
//     redirect, validates the state parameter, renders a styled success
//     or error page, and delivers the result on a channel.
//   - [ParseAuthorizationInput] — robust parser for codes the user pastes
//     manually (full URLs, code#state, query strings, or bare codes).
//   - [AwaitAuthCode] — races the loopback callback against an optional
//     manual-paste prompt; the first arrival wins.
//   - [ExchangeCode] / [Refresh] — stateless token-endpoint primitives
//     (RFC 6749 §4.1.3 + §6) returning a parsed [Token] whose [Token.Raw]
//     map preserves every provider-specific field. Errors come back as
//     [*TokenError] (RFC 6749 §5.2).
//
// The [Provider] interface is a convention for assembling these pieces
// into provider-specific login flows; pinoauth itself ships no concrete
// providers.
//
// # Non-goals
//
// pinoauth deliberately does NOT provide:
//
//   - A TokenSource interface, auto-refreshing http.Client, or
//     RoundTripper that injects bearer tokens.
//   - Background refresh goroutines or any concurrency primitive
//     around tokens.
//   - Token storage, on-disk persistence, or keychain integration.
//
// Token lifetime, refresh timing, persistence, and concurrency are the
// caller's concern — typically a thin layer in the consuming app that
// already knows how it wants to store credentials. Use [Token.Expired]
// or [Token.ExpiresWithin] to decide when to call [Refresh].
//
// pinoauth was extracted from the fir coding-agent harness
// (https://github.com/kfet/fir), where it powers OAuth login for
// multiple providers.
package pinoauth
