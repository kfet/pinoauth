package pinoauth

import (
	"context"
	_ "embed"
	"fmt"
	"html"
	"net"
	"net/http"
	"strings"
	"sync"
)

//go:embed callback_page.html
var callbackPageHTML string

// Placeholder tokens used in callback_page.html.
const (
	phTitle   = "__TITLE__"
	phIcon    = "__ICON__"
	phHeading = "__HEADING__"
	phMessage = "__MESSAGE__"
)

// allPlaceholders is the complete set of tokens that renderAuthPage replaces.
// A test asserts every placeholder appears in the embedded HTML so the two
// stay in sync.
var allPlaceholders = []string{phTitle, phIcon, phHeading, phMessage}

// renderAuthPage renders the callback page with the given content.
// Uses simple string replacement to avoid html/template (which adds ~6MB to
// the binary due to reflect-driven linker retention of large indirect deps).
func renderAuthPage(title, icon, heading, message string) string {
	r := strings.NewReplacer(
		phTitle, html.EscapeString(title),
		phIcon, icon,
		phHeading, html.EscapeString(heading),
		phMessage, html.EscapeString(message),
	)
	return r.Replace(callbackPageHTML)
}

// CallbackResult is the authorization data delivered by the loopback server
// after a successful redirect.
type CallbackResult struct {
	// Code is the OAuth 2.0 authorization code (RFC 6749 §4.1.2).
	Code string
	// State is the state value echoed back by the authorization server.
	// When [StartCallbackServer] is called with a non-empty expectedState,
	// State is guaranteed to equal it.
	State string
}

// StartCallbackServer starts a loopback HTTP server to receive an OAuth 2.0
// authorization-code redirect (RFC 8252 §7.3).
//
// route is the path to listen on (e.g. "/oauth-callback"). addr is the
// listener address (e.g. "127.0.0.1:0" to pick a free port). expectedState,
// if non-empty, is validated server-side: requests with a mismatched state
// receive HTTP 400 and are not delivered on the result channel.
//
// On success it returns the running [http.Server], a receive-only channel
// carrying at most one [CallbackResult], and the resolved listener address
// (which differs from addr when port 0 was requested).
//
// Lifecycle:
//
//   - The result channel is buffered (capacity 1). A successful callback
//     sends exactly one *CallbackResult and the channel is left open;
//     subsequent callbacks are not delivered.
//   - When ctx is cancelled the server is closed and the channel is closed
//     (a closed channel yields a nil *CallbackResult, distinguishable from
//     a real result).
//   - If the underlying [http.Server.Serve] fails for any reason other than
//     [http.ErrServerClosed], the channel is closed.
//   - Callers should still defer srv.Close() (or srv.Shutdown) to release
//     the listener if they exit before ctx is cancelled.
//
// The returned server and channel are safe for concurrent use; the channel
// has a single producer (the request handler goroutine) and any number of
// receivers.
func StartCallbackServer(ctx context.Context, route, addr, expectedState string) (server *http.Server, resultCh <-chan *CallbackResult, actualAddr string, err error) {
	ch := make(chan *CallbackResult, 1)
	var once sync.Once

	mux := http.NewServeMux()
	mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		errParam := r.URL.Query().Get("error")
		if errParam != "" {
			w.WriteHeader(400)
			fmt.Fprint(w, renderAuthPage("Authentication Failed", "⚠️", "Authentication Failed", "Error: "+errParam))
			return
		}

		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")

		if expectedState != "" && state != expectedState {
			w.WriteHeader(400)
			fmt.Fprint(w, renderAuthPage("Authentication Failed", "🚫", "Authentication Failed", "State mismatch — please try again."))
			return
		}

		if code != "" {
			fmt.Fprint(w, renderAuthPage("Authentication Successful", "✓", "You're all set", "You can close this window and return to the terminal."))
			once.Do(func() {
				ch <- &CallbackResult{Code: code, State: state}
			})
		} else {
			w.WriteHeader(400)
			fmt.Fprint(w, renderAuthPage("Authentication Failed", "⚠️", "Authentication Failed", "Missing authorization code."))
		}
	})

	srv := &http.Server{Handler: mux}

	ln, listenErr := net.Listen("tcp", addr)
	if listenErr != nil {
		return nil, nil, "", fmt.Errorf("pinoauth: listen on %s: %w", addr, listenErr)
	}

	resolvedAddr := ln.Addr().String()

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			once.Do(func() { close(ch) })
		}
	}()

	go func() {
		<-ctx.Done()
		srv.Close()
		once.Do(func() { close(ch) })
	}()

	return srv, ch, resolvedAddr, nil
}
