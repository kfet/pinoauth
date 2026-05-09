# pinoauth

<!-- TODO(badges): once the GitHub repo is published, add:
       - CI status:    ![CI](https://github.com/kfet/pinoauth/actions/workflows/test.yml/badge.svg)
       - pkg.go.dev:   [![Go Reference](https://pkg.go.dev/badge/github.com/kfet/pinoauth.svg)](https://pkg.go.dev/github.com/kfet/pinoauth)
       - Go report:    [![Go Report Card](https://goreportcard.com/badge/github.com/kfet/pinoauth)](https://goreportcard.com/report/github.com/kfet/pinoauth)
-->

A small, **stdlib-only** Go toolkit for browser-loopback OAuth 2.0 flows in
CLI and desktop applications — i.e. [RFC 8252](https://datatracker.ietf.org/doc/html/rfc8252)
"OAuth 2.0 for Native Apps" with [PKCE](https://datatracker.ietf.org/doc/html/rfc7636).

Extracted from the [fir](https://github.com/kfet/fir) coding-agent harness,
where it handles login for multiple providers.

## What it is

The pieces every native-app PKCE flow needs, and nothing else:

- **PKCE** — `GeneratePKCE()` returns a 32-byte random verifier and its
  base64url-encoded SHA-256 challenge.
- **Loopback callback server** — `StartCallbackServer()` binds a port
  on `127.0.0.1`, listens for the redirect, validates the `state`
  parameter, renders a styled HTML success/error page, and delivers
  `{Code, State}` on a channel.
- **Pasted-code parser** — `ParseAuthorizationInput()` robustly extracts
  `code` and `state` from whatever the user pastes back from the browser:
  a full callback URL, `code#state` (the OpenAI-style "manual entry"
  form), a `code=…&state=…` query fragment, or a bare code.
- **Callback / paste race** — `AwaitAuthCode()` waits for either the
  loopback callback or a manual paste, whichever arrives first.
  Composes the three primitives above into the SSH-friendly fallback
  every native-app OAuth flow needs in practice.
- **Token endpoint** — `ExchangeCode()` and `Refresh()` POST
  `grant_type=authorization_code` / `grant_type=refresh_token` to a
  provider's token endpoint, parse the JSON response into a `Token`
  (with `ExpiresAt` computed at receive time), and surface
  RFC 6749 §5.2 errors as `*TokenError`. Stateless: no auto-refresh,
  no goroutines, no storage. The provider-specific fields a non-trivial
  flow needs (`id_token`, account IDs, even non-standard top-level
  shapes) are preserved verbatim in `Token.Raw`.

Plus a `Provider` interface that's a convention for assembling these into
a provider-specific login flow. `pinoauth` ships **no concrete providers** —
those live in your code.

## What it isn't

- Not a full OAuth client/server framework.
- Not a token store. Persistence is your problem.
- Not for browser-based / SPA / confidential-client flows. Loopback only.
- **No `TokenSource`, no auto-refreshing `http.Client`, no background
  goroutines.** `Token` is a plain value; the caller decides when to
  refresh (`Token.ExpiresWithin(5*time.Minute)`) and where to store
  the result.

## Install

```bash
go get github.com/kfet/pinoauth
```

## Quick start

```go
package main

import (
    "context"
    "fmt"
    "net/url"

    "github.com/kfet/pinoauth"
)

func main() {
    ctx := context.Background()

    // 1. PKCE.
    pkce, _ := pinoauth.GeneratePKCE()

    // 2. Spin up the loopback callback server.
    state := "random-state-token"  // bring your own RNG
    srv, resultCh, addr, err := pinoauth.StartCallbackServer(
        ctx, "/callback", "127.0.0.1:0", state,
    )
    if err != nil { panic(err) }
    defer srv.Shutdown(ctx)

    // 3. Build the authorization URL with redirect_uri pointing at addr.
    redirect := "http://" + addr + "/callback"
    authURL := "https://example.com/oauth/authorize?" + url.Values{
        "client_id":             {"YOUR_CLIENT_ID"},
        "redirect_uri":          {redirect},
        "response_type":         {"code"},
        "code_challenge":        {pkce.Challenge},
        "code_challenge_method": {"S256"},
        "state":                 {state},
    }.Encode()

    fmt.Println("Open in your browser:", authURL)

    // 4. Wait for the callback.
    res := <-resultCh
    fmt.Printf("Got code=%s state=%s\n", res.Code, res.State)

    // 5. Exchange res.Code + pkce.Verifier for tokens at the provider's
    //    /token endpoint. (That part is provider-specific — your code.)
}
```

For the manual-paste fallback (when the browser can't reach `127.0.0.1`,
e.g. SSH sessions), call `pinoauth.ParseAuthorizationInput(pasted)` on the
text the user provides.

## Stability

`v0.x` — the API is in flux but I try to avoid pointless churn. The
extracted-from-fir surface has been stable for many months.

## License

[MIT](LICENSE)
