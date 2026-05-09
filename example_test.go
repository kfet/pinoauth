package pinoauth_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/kfet/pinoauth"
)

// ExampleStartCallbackServer demonstrates a minimal loopback flow: spin up
// the callback server, build the authorization URL, and wait for the
// browser redirect. Here the "browser" is simulated with an http.Get so
// the example is fully self-contained and runnable.
func ExampleStartCallbackServer() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pkce := pinoauth.GeneratePKCE()
	state := pinoauth.GenerateState()
	srv, resultCh, addr, err := pinoauth.StartCallbackServer(
		ctx, "/callback", "127.0.0.1:0", state,
	)
	if err != nil {
		panic(err)
	}
	defer srv.Close()

	// In a real flow you'd open this URL in the user's browser.
	authURL := "https://example.com/oauth/authorize?" + url.Values{
		"client_id":             {"YOUR_CLIENT_ID"},
		"redirect_uri":          {"http://" + addr + "/callback"},
		"response_type":         {"code"},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
	}.Encode()
	_ = authURL

	// Simulate the browser redirect hitting the loopback server.
	go func() {
		resp, err := http.Get("http://" + addr + "/callback?" + url.Values{
			"code":  {"sample-auth-code"},
			"state": {state},
		}.Encode())
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}()

	res := <-resultCh
	fmt.Printf("code=%s state_matches=%v\n", res.Code, res.State == state)
	// Output: code=sample-auth-code state_matches=true
}
