package pinoauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// echoServer accepts a token request, validates the form/json body,
// and returns a canned response.
type echoServer struct {
	t              *testing.T
	expectJSON     bool
	wantFields     map[string]string
	wantHeader     map[string]string
	respStatus     int
	respBody       string
	respContent    string // override content type
	gotFormCapture *url.Values
}

func (e *echoServer) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		e.t.Errorf("expected POST, got %s", r.Method)
	}
	for k, v := range e.wantHeader {
		if got := r.Header.Get(k); got != v {
			e.t.Errorf("header %s: want %q got %q", k, v, got)
		}
	}
	body, _ := io.ReadAll(r.Body)
	var fields url.Values
	if e.expectJSON {
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			e.t.Errorf("expected JSON content type, got %q", ct)
		}
		flat := map[string]string{}
		if err := json.Unmarshal(body, &flat); err != nil {
			e.t.Errorf("body not JSON: %v (%s)", err, body)
		}
		fields = url.Values{}
		for k, v := range flat {
			fields.Set(k, v)
		}
	} else {
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
			e.t.Errorf("expected form content type, got %q", ct)
		}
		var err error
		fields, err = url.ParseQuery(string(body))
		if err != nil {
			e.t.Errorf("bad form body: %v", err)
		}
	}
	for k, v := range e.wantFields {
		if got := fields.Get(k); got != v {
			e.t.Errorf("field %s: want %q got %q", k, v, got)
		}
	}
	if e.gotFormCapture != nil {
		*e.gotFormCapture = fields
	}
	if e.respContent != "" {
		w.Header().Set("Content-Type", e.respContent)
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	if e.respStatus != 0 {
		w.WriteHeader(e.respStatus)
	}
	io.WriteString(w, e.respBody)
}

// minimalExchangeReq is the standard ExchangeRequest used by tests that
// only care about the response side of the round-trip.
func minimalExchangeReq() ExchangeRequest {
	return ExchangeRequest{Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb"}
}

func TestClient_Exchange_StandardForm(t *testing.T) {
	es := &echoServer{
		t: t,
		wantFields: map[string]string{
			"grant_type":    "authorization_code",
			"client_id":     "cid",
			"code":          "the-code",
			"code_verifier": "the-verifier",
			"redirect_uri":  "http://127.0.0.1:1234/cb",
		},
		wantHeader: map[string]string{"User-Agent": "pinoauth-test"},
		respBody: `{"access_token":"AT","token_type":"Bearer","refresh_token":"RT",` +
			`"expires_in":3600,"scope":"a b","id_token":"jwt-here"}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(es.handler))
	defer srv.Close()

	c := &Client{
		TokenURL: srv.URL,
		ClientID: "cid",
		Headers:  http.Header{"User-Agent": {"pinoauth-test"}},
	}
	tok, err := c.Exchange(context.Background(), ExchangeRequest{
		Code:         "the-code",
		CodeVerifier: "the-verifier",
		RedirectURI:  "http://127.0.0.1:1234/cb",
	})
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if tok.AccessToken != "AT" {
		t.Errorf("AccessToken=%q", tok.AccessToken)
	}
	if tok.RefreshToken != "RT" {
		t.Errorf("RefreshToken=%q", tok.RefreshToken)
	}
	if tok.TokenType != "Bearer" {
		t.Errorf("TokenType=%q", tok.TokenType)
	}
	if tok.Scope != "a b" {
		t.Errorf("Scope=%q", tok.Scope)
	}
	if tok.ExpiresAt.IsZero() {
		t.Error("ExpiresAt zero")
	}
	if d := time.Until(tok.ExpiresAt); d < 50*time.Minute || d > 65*time.Minute {
		t.Errorf("ExpiresAt off: %v", d)
	}
	if got, _ := tok.Raw["id_token"].(string); got != "jwt-here" {
		t.Errorf("Raw[id_token]=%v", tok.Raw["id_token"])
	}
}

func TestClient_Exchange_JSONBody(t *testing.T) {
	es := &echoServer{
		t:          t,
		expectJSON: true,
		wantFields: map[string]string{
			"grant_type": "authorization_code",
			"client_id":  "cid",
			"code":       "C",
		},
		respBody: `{"access_token":"AT","refresh_token":"RT","expires_in":60}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(es.handler))
	defer srv.Close()

	c := &Client{TokenURL: srv.URL, ClientID: "cid", BodyEncoder: JSONBodyEncoder}
	tok, err := c.Exchange(context.Background(), minimalExchangeReq())
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if tok.AccessToken != "AT" || tok.RefreshToken != "RT" {
		t.Errorf("got %+v", tok)
	}
}

func TestClient_Exchange_ExtraNonReservedFields(t *testing.T) {
	var captured url.Values
	es := &echoServer{
		t:              t,
		gotFormCapture: &captured,
		respBody:       `{"access_token":"AT"}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(es.handler))
	defer srv.Close()

	c := &Client{TokenURL: srv.URL, ClientID: "cid"}
	_, err := c.Exchange(context.Background(), ExchangeRequest{
		Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
		Extra: url.Values{
			"audience": {"my-aud"},
			"state":    {"abc123"},
		},
	})
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if captured.Get("audience") != "my-aud" {
		t.Errorf("audience=%q", captured.Get("audience"))
	}
	if captured.Get("state") != "abc123" {
		t.Errorf("state=%q", captured.Get("state"))
	}
	if captured.Get("client_id") != "cid" {
		t.Errorf("client_id should be untouched, got %q", captured.Get("client_id"))
	}
}

func TestClient_Exchange_ExtraReservedKeyRejected(t *testing.T) {
	c := &Client{TokenURL: "http://x", ClientID: "cid"}
	for _, k := range []string{"grant_type", "client_id", "client_secret", "code", "code_verifier", "redirect_uri"} {
		_, err := c.Exchange(context.Background(), ExchangeRequest{
			Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
			Extra: url.Values{k: {"x"}},
		})
		if err == nil {
			t.Errorf("key %q: expected error, got nil", k)
			continue
		}
		if !strings.Contains(err.Error(), k) {
			t.Errorf("key %q: error %q does not name the key", k, err.Error())
		}
	}
}

func TestClient_Exchange_ExtraInJSONBody(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"access_token":"AT"}`)
	}))
	defer srv.Close()
	c := &Client{TokenURL: srv.URL, ClientID: "cid", BodyEncoder: JSONBodyEncoder}
	_, err := c.Exchange(context.Background(), ExchangeRequest{
		Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
		Extra: url.Values{"state": {"S123"}},
	})
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	var parsed map[string]string
	if err := json.Unmarshal(gotBody, &parsed); err != nil {
		t.Fatalf("body not JSON: %v: %s", err, gotBody)
	}
	if parsed["state"] != "S123" {
		t.Errorf("state in JSON body=%q want S123 (body=%s)", parsed["state"], gotBody)
	}
}

func TestClient_Refresh_ExtraReservedKeyRejected(t *testing.T) {
	c := &Client{TokenURL: "http://x", ClientID: "cid"}
	for _, k := range []string{"grant_type", "client_id", "client_secret", "refresh_token", "scope"} {
		_, err := c.Refresh(context.Background(), RefreshRequest{
			RefreshToken: "rt",
			Extra:        url.Values{k: {"x"}},
		})
		if err == nil {
			t.Errorf("key %q: expected error, got nil", k)
			continue
		}
		if !strings.Contains(err.Error(), k) {
			t.Errorf("key %q: error %q does not name the key", k, err.Error())
		}
	}
}

func TestClient_Exchange_OAuthErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		io.WriteString(w, `{"error":"invalid_grant","error_description":"code expired","error_uri":"https://e.example/x"}`)
	}))
	defer srv.Close()

	c := &Client{TokenURL: srv.URL, ClientID: "cid"}
	_, err := c.Exchange(context.Background(), minimalExchangeReq())
	if err == nil {
		t.Fatal("expected error")
	}
	var te *TokenError
	if !errors.As(err, &te) {
		t.Fatalf("expected *TokenError, got %T: %v", err, err)
	}
	if te.Code != "invalid_grant" {
		t.Errorf("Code=%q", te.Code)
	}
	if te.Description != "code expired" {
		t.Errorf("Description=%q", te.Description)
	}
	if te.URI != "https://e.example/x" {
		t.Errorf("URI=%q", te.URI)
	}
	if te.HTTPStatus != 400 {
		t.Errorf("HTTPStatus=%d", te.HTTPStatus)
	}
	if !strings.Contains(te.Error(), "invalid_grant") {
		t.Errorf("Error()=%q", te.Error())
	}
}

func TestClient_Exchange_NonOAuthErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		io.WriteString(w, "<html>boom</html>")
	}))
	defer srv.Close()

	c := &Client{TokenURL: srv.URL, ClientID: "c"}
	_, err := c.Exchange(context.Background(), minimalExchangeReq())
	var te *TokenError
	if !errors.As(err, &te) {
		t.Fatalf("expected *TokenError, got %v", err)
	}
	if te.Code != "" {
		t.Errorf("Code should be empty, got %q", te.Code)
	}
	if !strings.Contains(string(te.Body), "boom") {
		t.Errorf("Body=%q", te.Body)
	}
	if te.HTTPStatus != 500 {
		t.Errorf("HTTPStatus=%d", te.HTTPStatus)
	}
}

func TestClient_Exchange_TransportError(t *testing.T) {
	// Closed server -> connection refused.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()
	c := &Client{TokenURL: srv.URL, ClientID: "c"}
	_, err := c.Exchange(context.Background(), minimalExchangeReq())
	var te *TokenError
	if !errors.As(err, &te) {
		t.Fatalf("expected *TokenError, got %v", err)
	}
	if te.Err == nil {
		t.Error("expected Err to wrap transport error")
	}
}

func TestClient_Exchange_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := &Client{TokenURL: srv.URL, ClientID: "c"}
	_, err := c.Exchange(ctx, minimalExchangeReq())
	if err == nil {
		t.Fatal("expected error from cancelled ctx")
	}
}

func TestClient_Exchange_RequiredParams(t *testing.T) {
	type tc struct {
		name string
		c    Client
		r    ExchangeRequest
	}
	cases := []tc{
		{"empty", Client{}, ExchangeRequest{}},
		{"no client id", Client{TokenURL: "x"}, ExchangeRequest{}},
		{"no code", Client{TokenURL: "x", ClientID: "c"}, ExchangeRequest{}},
		{"no verifier", Client{TokenURL: "x", ClientID: "c"}, ExchangeRequest{Code: "C"}},
		{"no redirect", Client{TokenURL: "x", ClientID: "c"}, ExchangeRequest{Code: "C", CodeVerifier: "V"}},
	}
	for _, tc := range cases {
		if _, err := tc.c.Exchange(context.Background(), tc.r); err == nil {
			t.Errorf("%s: expected validation error", tc.name)
		}
	}
}

func TestClient_Refresh_Standard(t *testing.T) {
	es := &echoServer{
		t: t,
		wantFields: map[string]string{
			"grant_type":    "refresh_token",
			"client_id":     "cid",
			"refresh_token": "old-rt",
		},
		respBody: `{"access_token":"AT2","refresh_token":"RT2","expires_in":120}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(es.handler))
	defer srv.Close()

	c := &Client{TokenURL: srv.URL, ClientID: "cid"}
	tok, err := c.Refresh(context.Background(), RefreshRequest{RefreshToken: "old-rt"})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tok.AccessToken != "AT2" || tok.RefreshToken != "RT2" {
		t.Errorf("got %+v", tok)
	}
	if tok.ExpiresAt.IsZero() {
		t.Error("ExpiresAt zero")
	}
}

func TestClient_Refresh_Required(t *testing.T) {
	type tc struct {
		name string
		c    Client
		r    RefreshRequest
	}
	cases := []tc{
		{"empty", Client{}, RefreshRequest{}},
		{"no client id", Client{TokenURL: "x"}, RefreshRequest{RefreshToken: "rt"}},
		{"no refresh token", Client{TokenURL: "x", ClientID: "c"}, RefreshRequest{}},
	}
	for _, tc := range cases {
		if _, err := tc.c.Refresh(context.Background(), tc.r); err == nil {
			t.Errorf("%s: expected error", tc.name)
		}
	}
}

func TestToken_Expired(t *testing.T) {
	if (&Token{}).Expired() {
		t.Error("zero-time token should not be expired")
	}
	if (&Token{ExpiresAt: time.Now().Add(time.Hour)}).Expired() {
		t.Error("future token should not be expired")
	}
	if !(&Token{ExpiresAt: time.Now().Add(-time.Second)}).Expired() {
		t.Error("past token should be expired")
	}
	var nilTok *Token
	if nilTok.Expired() {
		t.Error("nil token should not be expired")
	}
}

func TestToken_ExpiresWithin(t *testing.T) {
	if (&Token{}).ExpiresWithin(time.Hour) {
		t.Error("zero-time token: ExpiresWithin should be false")
	}
	if !(&Token{ExpiresAt: time.Now().Add(30 * time.Second)}).ExpiresWithin(time.Minute) {
		t.Error("token expiring in 30s should be within 1m")
	}
	if (&Token{ExpiresAt: time.Now().Add(time.Hour)}).ExpiresWithin(time.Minute) {
		t.Error("token expiring in 1h should not be within 1m")
	}
}

func TestExpiresIn_StringForm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"access_token":"A","expires_in":"3600"}`)
	}))
	defer srv.Close()
	c := &Client{TokenURL: srv.URL, ClientID: "c"}
	tok, err := c.Exchange(context.Background(), minimalExchangeReq())
	if err != nil {
		t.Fatal(err)
	}
	if tok.ExpiresAt.IsZero() {
		t.Error("string expires_in not parsed")
	}
}

func TestExpiresIn_Missing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"access_token":"A"}`)
	}))
	defer srv.Close()
	c := &Client{TokenURL: srv.URL, ClientID: "c"}
	tok, err := c.Exchange(context.Background(), minimalExchangeReq())
	if err != nil {
		t.Fatal(err)
	}
	if !tok.ExpiresAt.IsZero() {
		t.Error("missing expires_in should yield zero ExpiresAt")
	}
	if tok.Expired() {
		t.Error("zero ExpiresAt should not be Expired")
	}
}

func TestClient_Exchange_NonStandardResponseViaRaw(t *testing.T) {
	// Poe-style: returns "api_key" instead of "access_token".
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"api_key":"poe-key-xyz"}`)
	}))
	defer srv.Close()
	c := &Client{TokenURL: srv.URL, ClientID: "c"}
	tok, err := c.Exchange(context.Background(), minimalExchangeReq())
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "" {
		t.Errorf("AccessToken should be empty, got %q", tok.AccessToken)
	}
	if got, _ := tok.Raw["api_key"].(string); got != "poe-key-xyz" {
		t.Errorf("Raw[api_key]=%v", tok.Raw["api_key"])
	}
}

func TestJSONBodyEncoder(t *testing.T) {
	ct, body, err := JSONBodyEncoder(url.Values{"a": {"1"}, "b": {"2"}})
	if err != nil {
		t.Fatal(err)
	}
	if ct != "application/json" {
		t.Errorf("ct=%q", ct)
	}
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["a"] != "1" || got["b"] != "2" {
		t.Errorf("got %v", got)
	}
}

func TestTokenError_ErrorString(t *testing.T) {
	cases := []struct {
		te   TokenError
		want string
	}{
		{TokenError{Err: errors.New("boom")}, "pinoauth: token request failed: boom"},
		{TokenError{Code: "invalid_grant", Description: "no"}, "pinoauth: token endpoint error invalid_grant: no"},
		{TokenError{Code: "invalid_grant"}, "pinoauth: token endpoint error: invalid_grant"},
		{TokenError{HTTPStatus: 502}, "pinoauth: token request failed: HTTP 502"},
		{TokenError{}, "pinoauth: token request failed"},
	}
	for _, c := range cases {
		if got := c.te.Error(); got != c.want {
			t.Errorf("Error()=%q want %q", got, c.want)
		}
	}
	te := &TokenError{Err: errors.New("x")}
	if te.Unwrap() == nil {
		t.Error("Unwrap should return Err")
	}
}

func TestClient_Exchange_BodyEncoderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	c := &Client{
		TokenURL: srv.URL, ClientID: "c",
		BodyEncoder: func(url.Values) (string, []byte, error) {
			return "", nil, errors.New("encode-fail")
		},
	}
	_, err := c.Exchange(context.Background(), minimalExchangeReq())
	var te *TokenError
	if !errors.As(err, &te) || te.Err == nil {
		t.Fatalf("expected TokenError wrapping encode err, got %v", err)
	}
	if !strings.Contains(te.Err.Error(), "encode-fail") {
		t.Errorf("Err=%v", te.Err)
	}
}

func TestClient_Exchange_MalformedSuccessBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "not-json")
	}))
	defer srv.Close()
	c := &Client{TokenURL: srv.URL, ClientID: "c"}
	_, err := c.Exchange(context.Background(), minimalExchangeReq())
	var te *TokenError
	if !errors.As(err, &te) {
		t.Fatalf("expected TokenError, got %v", err)
	}
	if te.Err == nil {
		t.Error("expected wrapped json error")
	}
	if !strings.Contains(string(te.Body), "not-json") {
		t.Errorf("Body=%q", te.Body)
	}
}

func TestClient_Exchange_MidStreamReadError(t *testing.T) {
	// Custom RoundTripper returning a response whose body errors on read.
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{},
			Body:       io.NopCloser(errReader{err: errors.New("conn-reset")}),
			Request:    r,
		}, nil
	})
	c := &Client{
		TokenURL:   "http://example.invalid/",
		ClientID:   "c",
		HTTPClient: &http.Client{Transport: rt},
	}
	_, err := c.Exchange(context.Background(), minimalExchangeReq())
	var te *TokenError
	if !errors.As(err, &te) || te.Err == nil {
		t.Fatalf("expected wrapped read error, got %v", err)
	}
}

func TestClient_Exchange_ClientSecret(t *testing.T) {
	var captured url.Values
	es := &echoServer{t: t, gotFormCapture: &captured, respBody: `{"access_token":"A"}`}
	srv := httptest.NewServer(http.HandlerFunc(es.handler))
	defer srv.Close()
	c := &Client{TokenURL: srv.URL, ClientID: "c", ClientSecret: "shh"}
	_, err := c.Exchange(context.Background(), minimalExchangeReq())
	if err != nil {
		t.Fatal(err)
	}
	if captured.Get("client_secret") != "shh" {
		t.Errorf("client_secret missing: %v", captured)
	}
}

func TestClient_Refresh_AllOptions(t *testing.T) {
	var captured url.Values
	es := &echoServer{
		t:              t,
		gotFormCapture: &captured,
		wantHeader:     map[string]string{"X-Test": "yes"},
		respBody:       `{"access_token":"A","expires_in":10}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(es.handler))
	defer srv.Close()
	c := &Client{
		TokenURL: srv.URL, ClientID: "c", ClientSecret: "s",
		Headers:    http.Header{"X-Test": {"yes"}},
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}
	_, err := c.Refresh(context.Background(), RefreshRequest{
		RefreshToken: "rt", Scope: "a b",
		Extra: url.Values{"audience": {"x"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{
		"client_secret": "s", "scope": "a b", "audience": "x",
	} {
		if captured.Get(k) != v {
			t.Errorf("%s=%q want %q", k, captured.Get(k), v)
		}
	}
}

// roundTripperFunc adapts a func to http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func TestClient_Exchange_InvalidURLReturnsError(t *testing.T) {
	c := &Client{TokenURL: "http://\x7f/", ClientID: "c"} // CTL byte rejected by net/http
	_, err := c.Exchange(context.Background(), minimalExchangeReq())
	var te *TokenError
	if !errors.As(err, &te) {
		t.Fatalf("expected *TokenError, got %T: %v", err, err)
	}
	if te.Err == nil {
		t.Error("expected TokenError.Err to wrap URL parse failure")
	}
}

// TestClient_Exchange_DefaultClientRefusesRedirect verifies that the
// default HTTP client refuses to follow a 307/308 from the token
// endpoint, which would otherwise re-POST the body (including any
// client_secret / refresh_token / code_verifier) to the redirect
// target.
func TestClient_Exchange_DefaultClientRefusesRedirect(t *testing.T) {
	var hits int32
	var redirectURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.URL.Path == "/sink" {
			t.Errorf("body re-sent to redirect target — secrets would have leaked")
			return
		}
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	redirectURL = srv.URL + "/sink"

	c := &Client{TokenURL: srv.URL, ClientID: "c", ClientSecret: "secret-must-not-leak"}
	_, err := c.Exchange(context.Background(), minimalExchangeReq())
	if err == nil {
		t.Fatal("expected error on redirect")
	}
	if !errors.Is(err, ErrRedirectNotAllowed) {
		t.Errorf("expected errors.Is(err, ErrRedirectNotAllowed), got %v", err)
	}
	var te *TokenError
	if !errors.As(err, &te) {
		t.Errorf("expected *TokenError, got %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("expected exactly 1 server hit (redirect refused), got %d", hits)
	}
}

// TestClient_Exchange_BodySizeLimit verifies oversized responses are
// rejected rather than buffered unboundedly.
func TestClient_Exchange_BodySizeLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 2 MiB of JSON-ish junk → exceeds 1 MiB cap; truncated read
		// produces invalid JSON, surfacing as a decode error rather
		// than silently allocating 2 MiB.
		w.Write([]byte(`{"access_token":"`))
		filler := bytes.Repeat([]byte("x"), 2<<20)
		w.Write(filler)
		w.Write([]byte(`"}`))
	}))
	defer srv.Close()
	c := &Client{TokenURL: srv.URL, ClientID: "c"}
	_, err := c.Exchange(context.Background(), minimalExchangeReq())
	if err == nil {
		t.Fatal("expected error from oversized response")
	}
	var te *TokenError
	if !errors.As(err, &te) {
		t.Fatalf("expected *TokenError, got %v", err)
	}
	if len(te.Body) > maxTokenResponseBytes {
		t.Errorf("body buffered beyond cap: %d bytes", len(te.Body))
	}
}

// TestExpiresIn_HostileOverflow verifies that a server returning an
// absurd expires_in does not overflow the time.Duration math and
// produce a past-dated ExpiresAt.
func TestExpiresIn_HostileOverflow(t *testing.T) {
	for _, body := range []string{
		`{"access_token":"A","expires_in":1e18}`,
		`{"access_token":"A","expires_in":"1000000000000000000"}`,
	} {
		body := body
		t.Run(body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, body)
			}))
			defer srv.Close()
			c := &Client{TokenURL: srv.URL, ClientID: "c"}
			tok, err := c.Exchange(context.Background(), minimalExchangeReq())
			if err != nil {
				t.Fatal(err)
			}
			if tok.Expired() {
				t.Errorf("hostile expires_in must not yield already-expired token (ExpiresAt=%v)", tok.ExpiresAt)
			}
			if !tok.ExpiresAt.IsZero() && tok.ExpiresAt.Before(time.Now()) {
				t.Errorf("ExpiresAt before now (overflow): %v", tok.ExpiresAt)
			}
		})
	}
}

// TestClient_Exchange_HeaderContentTypeIgnored verifies caller-supplied
// Content-Type does not duplicate or override the BodyEncoder's value.
func TestClient_Exchange_HeaderContentTypeIgnored(t *testing.T) {
	var seenCT []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCT = r.Header.Values("Content-Type")
		io.WriteString(w, `{"access_token":"A"}`)
	}))
	defer srv.Close()
	c := &Client{
		TokenURL: srv.URL, ClientID: "c",
		Headers: http.Header{"Content-Type": {"text/plain"}, "X-Other": {"keep"}},
	}
	_, err := c.Exchange(context.Background(), minimalExchangeReq())
	if err != nil {
		t.Fatal(err)
	}
	if len(seenCT) != 1 || !strings.HasPrefix(seenCT[0], "application/x-www-form-urlencoded") {
		t.Errorf("Content-Type leaked from caller Headers: %v", seenCT)
	}
}
