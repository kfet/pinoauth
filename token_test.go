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

func TestExchangeCode_StandardForm(t *testing.T) {
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

	tok, err := ExchangeCode(context.Background(), ExchangeParams{
		TokenURL:     srv.URL,
		ClientID:     "cid",
		Code:         "the-code",
		CodeVerifier: "the-verifier",
		RedirectURI:  "http://127.0.0.1:1234/cb",
		Headers:      http.Header{"User-Agent": {"pinoauth-test"}},
	})
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
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

func TestExchangeCode_JSONBody(t *testing.T) {
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

	tok, err := ExchangeCode(context.Background(), ExchangeParams{
		TokenURL:     srv.URL,
		ClientID:     "cid",
		Code:         "C",
		CodeVerifier: "V",
		RedirectURI:  "http://x/cb",
		BodyEncoder:  JSONBodyEncoder,
	})
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tok.AccessToken != "AT" || tok.RefreshToken != "RT" {
		t.Errorf("got %+v", tok)
	}
}

func TestExchangeCode_ExtraFieldsOverride(t *testing.T) {
	var captured url.Values
	es := &echoServer{
		t:              t,
		gotFormCapture: &captured,
		respBody:       `{"access_token":"AT"}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(es.handler))
	defer srv.Close()

	_, err := ExchangeCode(context.Background(), ExchangeParams{
		TokenURL:     srv.URL,
		ClientID:     "cid",
		Code:         "C",
		CodeVerifier: "V",
		RedirectURI:  "http://x/cb",
		Extra: url.Values{
			"audience":  {"my-aud"},
			"client_id": {"override"},
		},
	})
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if captured.Get("audience") != "my-aud" {
		t.Errorf("audience=%q", captured.Get("audience"))
	}
	if captured.Get("client_id") != "override" {
		t.Errorf("client_id should be overridden, got %q", captured.Get("client_id"))
	}
}

func TestExchangeCode_OAuthErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		io.WriteString(w, `{"error":"invalid_grant","error_description":"code expired","error_uri":"https://e.example/x"}`)
	}))
	defer srv.Close()

	_, err := ExchangeCode(context.Background(), ExchangeParams{
		TokenURL:     srv.URL,
		ClientID:     "cid",
		Code:         "C",
		CodeVerifier: "V",
		RedirectURI:  "http://x/cb",
	})
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

func TestExchangeCode_NonOAuthErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		io.WriteString(w, "<html>boom</html>")
	}))
	defer srv.Close()

	_, err := ExchangeCode(context.Background(), ExchangeParams{
		TokenURL: srv.URL, ClientID: "c", Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
	})
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

func TestExchangeCode_TransportError(t *testing.T) {
	// Closed server -> connection refused.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()
	_, err := ExchangeCode(context.Background(), ExchangeParams{
		TokenURL: srv.URL, ClientID: "c", Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
	})
	var te *TokenError
	if !errors.As(err, &te) {
		t.Fatalf("expected *TokenError, got %v", err)
	}
	if te.Err == nil {
		t.Error("expected Err to wrap transport error")
	}
}

func TestExchangeCode_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ExchangeCode(ctx, ExchangeParams{
		TokenURL: srv.URL, ClientID: "c", Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
	})
	if err == nil {
		t.Fatal("expected error from cancelled ctx")
	}
}

func TestExchangeCode_RequiredParams(t *testing.T) {
	cases := []ExchangeParams{
		{},
		{TokenURL: "x"},
		{TokenURL: "x", ClientID: "c"},
		{TokenURL: "x", ClientID: "c", Code: "C"},
		{TokenURL: "x", ClientID: "c", Code: "C", CodeVerifier: "V"},
	}
	for i, p := range cases {
		if _, err := ExchangeCode(context.Background(), p); err == nil {
			t.Errorf("case %d: expected validation error", i)
		}
	}
}

func TestRefresh_Standard(t *testing.T) {
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

	tok, err := Refresh(context.Background(), RefreshParams{
		TokenURL:     srv.URL,
		ClientID:     "cid",
		RefreshToken: "old-rt",
	})
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

func TestRefresh_Required(t *testing.T) {
	if _, err := Refresh(context.Background(), RefreshParams{}); err == nil {
		t.Error("expected error for empty params")
	}
	if _, err := Refresh(context.Background(), RefreshParams{TokenURL: "x", ClientID: "c"}); err == nil {
		t.Error("expected error for missing refresh token")
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
	tok, err := ExchangeCode(context.Background(), ExchangeParams{
		TokenURL: srv.URL, ClientID: "c", Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
	})
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
	tok, err := ExchangeCode(context.Background(), ExchangeParams{
		TokenURL: srv.URL, ClientID: "c", Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
	})
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

func TestExchangeCode_NonStandardResponseViaRaw(t *testing.T) {
	// Poe-style: returns "api_key" instead of "access_token".
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"api_key":"poe-key-xyz"}`)
	}))
	defer srv.Close()
	tok, err := ExchangeCode(context.Background(), ExchangeParams{
		TokenURL: srv.URL, ClientID: "c", Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
	})
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

func TestExchangeCode_BodyEncoderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	_, err := ExchangeCode(context.Background(), ExchangeParams{
		TokenURL: srv.URL, ClientID: "c", Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
		BodyEncoder: func(url.Values) (string, []byte, error) {
			return "", nil, errors.New("encode-fail")
		},
	})
	var te *TokenError
	if !errors.As(err, &te) || te.Err == nil {
		t.Fatalf("expected TokenError wrapping encode err, got %v", err)
	}
	if !strings.Contains(te.Err.Error(), "encode-fail") {
		t.Errorf("Err=%v", te.Err)
	}
}

func TestExchangeCode_MalformedSuccessBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "not-json")
	}))
	defer srv.Close()
	_, err := ExchangeCode(context.Background(), ExchangeParams{
		TokenURL: srv.URL, ClientID: "c", Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
	})
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

func TestExchangeCode_MidStreamReadError(t *testing.T) {
	// Custom RoundTripper returning a response whose body errors on read.
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{},
			Body:       io.NopCloser(errReader{err: errors.New("conn-reset")}),
			Request:    r,
		}, nil
	})
	_, err := ExchangeCode(context.Background(), ExchangeParams{
		TokenURL: "http://example.invalid/",
		ClientID: "c", Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
		HTTPClient: &http.Client{Transport: rt},
	})
	var te *TokenError
	if !errors.As(err, &te) || te.Err == nil {
		t.Fatalf("expected wrapped read error, got %v", err)
	}
}

func TestExchangeCode_ClientSecret(t *testing.T) {
	var captured url.Values
	es := &echoServer{t: t, gotFormCapture: &captured, respBody: `{"access_token":"A"}`}
	srv := httptest.NewServer(http.HandlerFunc(es.handler))
	defer srv.Close()
	_, err := ExchangeCode(context.Background(), ExchangeParams{
		TokenURL: srv.URL, ClientID: "c", ClientSecret: "shh",
		Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
	})
	if err != nil {
		t.Fatal(err)
	}
	if captured.Get("client_secret") != "shh" {
		t.Errorf("client_secret missing: %v", captured)
	}
}

func TestRefresh_AllOptions(t *testing.T) {
	var captured url.Values
	es := &echoServer{
		t:              t,
		gotFormCapture: &captured,
		wantHeader:     map[string]string{"X-Test": "yes"},
		respBody:       `{"access_token":"A","expires_in":10}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(es.handler))
	defer srv.Close()
	_, err := Refresh(context.Background(), RefreshParams{
		TokenURL: srv.URL, ClientID: "c", ClientSecret: "s",
		RefreshToken: "rt", Scope: "a b",
		Extra:      url.Values{"audience": {"x"}},
		Headers:    http.Header{"X-Test": {"yes"}},
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
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

func TestRefresh_RequiredAll(t *testing.T) {
	cases := []RefreshParams{
		{},
		{TokenURL: "x"},
		{TokenURL: "x", ClientID: "c"},
	}
	for i, p := range cases {
		if _, err := Refresh(context.Background(), p); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
}

// roundTripperFunc adapts a func to http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func TestExchangeCode_InvalidURLReturnsError(t *testing.T) {
	_, err := ExchangeCode(context.Background(), ExchangeParams{
		TokenURL: "http://\x7f/", // CTL byte rejected by net/http
		ClientID: "c", Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
	})
	var te *TokenError
	if !errors.As(err, &te) {
		t.Fatalf("expected *TokenError, got %T: %v", err, err)
	}
	if te.Err == nil {
		t.Error("expected TokenError.Err to wrap URL parse failure")
	}
}

// TestExchangeCode_DefaultClientRefusesRedirect verifies that the
// default HTTP client refuses to follow a 307/308 from the token
// endpoint, which would otherwise re-POST the body (including any
// client_secret / refresh_token / code_verifier) to the redirect
// target.
func TestExchangeCode_DefaultClientRefusesRedirect(t *testing.T) {
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

	_, err := ExchangeCode(context.Background(), ExchangeParams{
		TokenURL: srv.URL, ClientID: "c", Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
		ClientSecret: "secret-must-not-leak",
	})
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

// TestExchangeCode_BodySizeLimit verifies oversized responses are
// rejected rather than buffered unboundedly.
func TestExchangeCode_BodySizeLimit(t *testing.T) {
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
	_, err := ExchangeCode(context.Background(), ExchangeParams{
		TokenURL: srv.URL, ClientID: "c", Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
	})
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
// produce a past-dated ExpiresAt (which would make Token.Expired()
// return true immediately and could drive a refresh loop).
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
			tok, err := ExchangeCode(context.Background(), ExchangeParams{
				TokenURL: srv.URL, ClientID: "c", Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
			})
			if err != nil {
				t.Fatal(err)
			}
			if tok.Expired() {
				t.Errorf("hostile expires_in must not yield already-expired token (ExpiresAt=%v)", tok.ExpiresAt)
			}
			// Either clamped to far-future or rejected to zero — both
			// are safe. What we forbid is a past-dated ExpiresAt from
			// integer-overflow.
			if !tok.ExpiresAt.IsZero() && tok.ExpiresAt.Before(time.Now()) {
				t.Errorf("ExpiresAt before now (overflow): %v", tok.ExpiresAt)
			}
		})
	}
}

// TestExchangeCode_HeaderContentTypeIgnored verifies caller-supplied
// Content-Type does not duplicate or override the BodyEncoder's value.
func TestExchangeCode_HeaderContentTypeIgnored(t *testing.T) {
	var seenCT []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCT = r.Header.Values("Content-Type")
		io.WriteString(w, `{"access_token":"A"}`)
	}))
	defer srv.Close()
	_, err := ExchangeCode(context.Background(), ExchangeParams{
		TokenURL: srv.URL, ClientID: "c", Code: "C", CodeVerifier: "V", RedirectURI: "http://x/cb",
		Headers: http.Header{"Content-Type": {"text/plain"}, "X-Other": {"keep"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seenCT) != 1 || !strings.HasPrefix(seenCT[0], "application/x-www-form-urlencoded") {
		t.Errorf("Content-Type leaked from caller Headers: %v", seenCT)
	}
}
