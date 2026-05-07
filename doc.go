// Package pinoauth is a small, stdlib-only toolkit for building OAuth 2.0
// browser-loopback flows (RFC 8252, "OAuth 2.0 for Native Apps") in CLI and
// desktop applications.
//
// It provides the three pieces every native-app PKCE flow needs and
// nothing else:
//
//   - [GeneratePKCE] — RFC 7636 code verifier + S256 challenge.
//   - [StartCallbackServer] — loopback HTTP server that catches the
//     redirect, validates the state parameter, renders a styled success
//     or error page, and delivers the result on a channel.
//   - [ParseAuthorizationInput] — robust parser for codes the user pastes
//     manually (full URLs, code#state, query strings, or bare codes).
//
// The [Provider] interface is a convention for assembling these pieces
// into provider-specific login flows; pinoauth itself ships no concrete
// providers.
//
// pinoauth was extracted from the fir coding-agent harness
// (https://github.com/kfet/fir), where it powers OAuth login for
// Anthropic, GitHub Copilot, OpenAI Codex, Gemini CLI, and others.
package pinoauth
