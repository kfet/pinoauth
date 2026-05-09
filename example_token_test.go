package pinoauth_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/kfet/pinoauth"
)

// ExampleExchangeCode demonstrates the full Anthropic-equivalent flow
// in pure pinoauth: PKCE → loopback callback → token exchange → refresh.
// The token endpoint is stubbed with httptest so the example is
// self-contained.
func ExampleExchangeCode() {
	// --- stub Anthropic-style token endpoint (JSON body, JSON response) ---
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var resp string
		switch {
		case strings.Contains(string(body), `"grant_type":"authorization_code"`):
			resp = `{"access_token":"AT-1","refresh_token":"RT-1","expires_in":3600,"token_type":"Bearer"}`
		case strings.Contains(string(body), `"grant_type":"refresh_token"`):
			resp = `{"access_token":"AT-2","refresh_token":"RT-2","expires_in":3600,"token_type":"Bearer"}`
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, resp)
	}))
	defer tokenSrv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. PKCE.
	pkce, _ := pinoauth.GeneratePKCE()

	// 2. Loopback callback server.
	const state = "example-state"
	srv, resultCh, addr, _ := pinoauth.StartCallbackServer(ctx, "/cb", "127.0.0.1:0", state)
	defer srv.Close()
	redirectURI := "http://" + addr + "/cb"

	// 3. Build auth URL (would be opened in the user's browser).
	authURL := "https://claude.ai/oauth/authorize?" + url.Values{
		"client_id":             {"CID"},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
	}.Encode()
	_ = authURL

	// 4. Simulate the browser redirect.
	go func() {
		resp, _ := http.Get(redirectURI + "?code=AC&state=" + state)
		if resp != nil {
			resp.Body.Close()
		}
	}()
	cb := <-resultCh

	// 5. Exchange code → tokens. Anthropic's endpoint takes JSON.
	tok, err := pinoauth.ExchangeCode(ctx, pinoauth.ExchangeParams{
		TokenURL:     tokenSrv.URL,
		ClientID:     "CID",
		Code:         cb.Code,
		CodeVerifier: pkce.Verifier,
		RedirectURI:  redirectURI,
		BodyEncoder:  pinoauth.JSONBodyEncoder,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("access=%s refresh=%s expires_set=%v\n",
		tok.AccessToken, tok.RefreshToken, !tok.ExpiresAt.IsZero())

	// 6. Later, when about to expire, refresh.
	if tok.ExpiresWithin(time.Hour + time.Minute) {
		tok2, err := pinoauth.Refresh(ctx, pinoauth.RefreshParams{
			TokenURL:     tokenSrv.URL,
			ClientID:     "CID",
			RefreshToken: tok.RefreshToken,
			BodyEncoder:  pinoauth.JSONBodyEncoder,
		})
		if err != nil {
			panic(err)
		}
		fmt.Printf("refreshed=%s\n", tok2.AccessToken)
	}

	// Output:
	// access=AT-1 refresh=RT-1 expires_set=true
	// refreshed=AT-2
}
