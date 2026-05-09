package pinoauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Token is a parsed OAuth 2.0 token response (RFC 6749 §5.1).
//
// Token is a plain data carrier; concurrent use must be guarded by the
// caller. Provider-specific fields not modelled by named members
// (id_token, account_id, api_key, …) are preserved verbatim in [Token.Raw]
// so callers can extract them without a second round-trip.
type Token struct {
	// AccessToken is the bearer access token (RFC 6749 §5.1
	// "access_token"). Set when the server returned a standard token
	// response; empty for providers that respond with a non-standard
	// shape (e.g. an "api_key" field instead) — read [Token.Raw] in
	// that case.
	AccessToken string
	// TokenType is the token type returned by the server, typically
	// "Bearer" (RFC 6749 §5.1 "token_type").
	TokenType string
	// RefreshToken is the refresh token (RFC 6749 §5.1
	// "refresh_token"); empty when the server did not issue one.
	RefreshToken string
	// ExpiresAt is the wall-clock expiry, computed at receive time as
	// now + expires_in. Zero when the response omits "expires_in"; in
	// that case [Token.Expired] returns false.
	ExpiresAt time.Time
	// Scope is the granted scope (RFC 6749 §5.1 "scope"); may be empty.
	Scope string
	// Raw is every top-level field of the JSON token response, decoded
	// as a generic map. Use this to read provider-specific fields such
	// as id_token, chatgpt_account_id, or api_key without a second
	// HTTP round-trip.
	Raw map[string]any
}

// Expired reports whether the token's ExpiresAt is in the past. It
// returns false when ExpiresAt is the zero value (server did not provide
// an expiry).
func (t *Token) Expired() bool {
	if t == nil || t.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(t.ExpiresAt)
}

// ExpiresWithin reports whether the token's ExpiresAt is within d of
// now (i.e. about to expire or already expired). Returns false when
// ExpiresAt is the zero value.
func (t *Token) ExpiresWithin(d time.Duration) bool {
	if t == nil || t.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().Add(d).After(t.ExpiresAt)
}

// TokenError is a token-endpoint failure: either an RFC 6749 §5.2 error
// response from the server (Code/Description/URI populated) or an
// underlying error (transport, body encoding, response decoding) wrapped
// via Err. Reachable via [errors.As].
type TokenError struct {
	// Code is the RFC 6749 §5.2 "error" field (e.g. "invalid_grant").
	// Empty when Err is non-nil or the response body could not be
	// parsed as a standard OAuth error response.
	Code string
	// Description is the RFC 6749 §5.2 "error_description" field.
	Description string
	// URI is the RFC 6749 §5.2 "error_uri" field.
	URI string
	// HTTPStatus is the response status code, or 0 when no response
	// reached us (DNS, connection refused, body-encode failure, ctx
	// cancellation before send).
	HTTPStatus int
	// Body is the raw response body when it could not be parsed as a
	// standard OAuth error response, or when a 2xx response body
	// failed to decode as JSON. Empty when Code is set.
	//
	// SECURITY: when a 2xx body fails to decode, Body holds the raw
	// bytes the server sent — which on a malformed-but-recognisable
	// response may include access_token / refresh_token material.
	// Do not log Body verbatim; redact or truncate before emission.
	// [TokenError.Error] never prints Body.
	Body []byte
	// Err is the underlying error wrapped by this TokenError —
	// transport (DNS, connection refused, I/O), body encoding (a
	// custom [BodyEncoder] returning an error), or response decoding
	// (malformed JSON in a 2xx body). Nil when the failure is a
	// well-formed RFC 6749 §5.2 error response. Use [errors.Is] /
	// [errors.Unwrap] to access.
	Err error
}

// Error implements the error interface.
func (e *TokenError) Error() string {
	switch {
	case e.Err != nil:
		return "pinoauth: token request failed: " + e.Err.Error()
	case e.Code != "" && e.Description != "":
		return fmt.Sprintf("pinoauth: token endpoint error %s: %s", e.Code, e.Description)
	case e.Code != "":
		return "pinoauth: token endpoint error: " + e.Code
	case e.HTTPStatus != 0:
		return fmt.Sprintf("pinoauth: token request failed: HTTP %d", e.HTTPStatus)
	default:
		return "pinoauth: token request failed"
	}
}

// Unwrap returns the wrapped underlying error (transport, encode, or
// decode), if any. See [TokenError.Err].
func (e *TokenError) Unwrap() error { return e.Err }

// BodyEncoder customises how token request parameters are serialised to
// the wire. The standard RFC 6749 encoding is application/x-www-form-
// urlencoded; the default (nil BodyEncoder) uses that. Providers that
// require a non-standard content type — e.g. JSON — can pass a custom
// encoder. See [JSONBodyEncoder].
//
// A non-nil err aborts the token request; the returned error is wrapped
// in a [*TokenError].
type BodyEncoder func(values url.Values) (contentType string, body []byte, err error)

// JSONBodyEncoder is a [BodyEncoder] that serialises parameters as a
// flat JSON object with application/json content type. Repeated form
// keys are collapsed to their first value. Provided for the small set
// of providers (notably Anthropic) that diverge from RFC 6749's form
// encoding at the token endpoint.
func JSONBodyEncoder(values url.Values) (string, []byte, error) {
	flat := make(map[string]string, len(values))
	for k, v := range values {
		if len(v) > 0 {
			flat[k] = v[0]
		}
	}
	// json.Marshal of a map[string]string cannot fail.
	body, _ := json.Marshal(flat)
	return "application/json", body, nil
}

// TokenClient is the contract for the token-endpoint half of an OAuth
// flow: an authorization-code → token exchange (RFC 6749 §4.1.3) and a
// refresh-token → token exchange (RFC 6749 §6). [*Client] is the
// canonical implementation; the interface lets callers swap in fakes for
// testing or compose alternate transports without depending on the
// concrete type.
type TokenClient interface {
	// Exchange performs an authorization-code grant.
	Exchange(ctx context.Context, req ExchangeRequest) (*Token, error)
	// Refresh performs a refresh-token grant.
	Refresh(ctx context.Context, req RefreshRequest) (*Token, error)
}

// Client is a configured OAuth token-endpoint client. The fields hold
// the per-provider configuration that does not change between requests
// (endpoint URL, credentials, transport, encoding); per-request data
// flows through [ExchangeRequest] and [RefreshRequest].
//
// TokenURL and ClientID are required. The zero value is not useful;
// construct a Client as a struct literal.
//
// Client is safe for concurrent use as long as callers do not mutate
// its fields after the first request.
type Client struct {
	// TokenURL is the provider's token endpoint. MUST be https:// in
	// production; the library does not enforce this so tests can use
	// httptest.NewServer.
	TokenURL string
	// ClientID is the OAuth client identifier.
	ClientID string
	// ClientSecret is sent as a form field when non-empty. Native apps
	// (RFC 8252) typically have no secret; leave empty in that case.
	ClientSecret string
	// HTTPClient overrides the default http.Client. When nil, an
	// internal client with a 30 s timeout is used.
	HTTPClient *http.Client
	// BodyEncoder overrides the default form encoding. See
	// [JSONBodyEncoder].
	BodyEncoder BodyEncoder
	// Headers are added to every HTTP request issued by this Client.
	// Content-Type is set by the BodyEncoder and must not be specified
	// here; a caller-supplied Content-Type is dropped.
	Headers http.Header
}

// Compile-time check that *Client satisfies TokenClient.
var _ TokenClient = (*Client)(nil)

// ExchangeRequest is the per-call input to [Client.Exchange]: the data
// from the authorization-endpoint round-trip that varies per login
// attempt. All fields except Extra are required.
type ExchangeRequest struct {
	// Code is the authorization code returned by the authorization
	// endpoint.
	Code string
	// CodeVerifier is the PKCE verifier (RFC 7636).
	CodeVerifier string
	// RedirectURI must match the redirect_uri sent in the authorization
	// request.
	RedirectURI string
	// Extra adds or overrides form fields on the request body. Useful
	// for provider-specific knobs (e.g. an audience parameter). Values
	// here override the standard fields when keys collide — including
	// security-critical fields such as grant_type, client_id, code,
	// code_verifier, and redirect_uri. Do not pipe untrusted input
	// into Extra.
	Extra url.Values
}

// RefreshRequest is the per-call input to [Client.Refresh]. RefreshToken
// is required; Scope and Extra are optional.
type RefreshRequest struct {
	// RefreshToken is the refresh token issued by a previous token
	// response.
	RefreshToken string
	// Scope optionally narrows the granted scope on refresh
	// (RFC 6749 §6 "scope"); leave empty to keep the original scope.
	Scope string
	// Extra adds or overrides form fields on the request body. See
	// [ExchangeRequest.Extra] — same caveats apply.
	Extra url.Values
}

// Exchange performs an authorization-code grant (RFC 6749 §4.1.3) with
// PKCE (RFC 7636) and returns the parsed [Token]. Any failure
// (transport, RFC 6749 §5.2 error response, body encoding, response
// decoding) is returned as a *[TokenError] — except missing required
// parameters, which return a plain error.
func (c *Client) Exchange(ctx context.Context, req ExchangeRequest) (*Token, error) {
	if c.TokenURL == "" {
		return nil, errors.New("pinoauth: Client.Exchange: TokenURL is required")
	}
	if c.ClientID == "" {
		return nil, errors.New("pinoauth: Client.Exchange: ClientID is required")
	}
	if req.Code == "" {
		return nil, errors.New("pinoauth: Client.Exchange: Code is required")
	}
	if req.CodeVerifier == "" {
		return nil, errors.New("pinoauth: Client.Exchange: CodeVerifier is required")
	}
	if req.RedirectURI == "" {
		return nil, errors.New("pinoauth: Client.Exchange: RedirectURI is required")
	}

	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("client_id", c.ClientID)
	values.Set("code", req.Code)
	values.Set("code_verifier", req.CodeVerifier)
	values.Set("redirect_uri", req.RedirectURI)
	if c.ClientSecret != "" {
		values.Set("client_secret", c.ClientSecret)
	}
	for k, vs := range req.Extra {
		values[k] = vs
	}

	return c.do(ctx, values)
}

// Refresh performs a refresh-token grant (RFC 6749 §6) and returns the
// parsed [Token]. Errors are returned the same way as [Client.Exchange]:
// any non-validation failure surfaces as a *[TokenError].
//
// Refresh is stateless: it does not mutate any input and does not
// persist tokens. The caller decides when to refresh (typically using
// [Token.ExpiresWithin]) and where to store the result.
func (c *Client) Refresh(ctx context.Context, req RefreshRequest) (*Token, error) {
	if c.TokenURL == "" {
		return nil, errors.New("pinoauth: Client.Refresh: TokenURL is required")
	}
	if c.ClientID == "" {
		return nil, errors.New("pinoauth: Client.Refresh: ClientID is required")
	}
	if req.RefreshToken == "" {
		return nil, errors.New("pinoauth: Client.Refresh: RefreshToken is required")
	}

	values := url.Values{}
	values.Set("grant_type", "refresh_token")
	values.Set("client_id", c.ClientID)
	values.Set("refresh_token", req.RefreshToken)
	if c.ClientSecret != "" {
		values.Set("client_secret", c.ClientSecret)
	}
	if req.Scope != "" {
		values.Set("scope", req.Scope)
	}
	for k, vs := range req.Extra {
		values[k] = vs
	}

	return c.do(ctx, values)
}

// ErrRedirectNotAllowed is returned (wrapped) when the token endpoint
// responds with a redirect. pinoauth's default HTTP client refuses to
// follow because a redirect on a token-endpoint POST would re-send the
// body — which carries client_secret, refresh_token, and code_verifier
// — to the redirect target. The error surfaces wrapped inside a
// *[url.Error] (from net/http) and then inside [TokenError.Err];
// [errors.Is] matches through both layers.
var ErrRedirectNotAllowed = errors.New("pinoauth: redirect not allowed on token endpoint")

// defaultHTTPClient is used when the caller does not supply one. We
// avoid http.DefaultClient because it has no timeout and follows
// redirects: a redirect on a token-endpoint POST would re-send the
// body (which carries client_secret, refresh_token, code_verifier)
// to the redirect target. Token endpoints do not redirect in
// practice, so we refuse rather than follow.
var defaultHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return ErrRedirectNotAllowed
	},
}

// maxTokenResponseBytes caps the size of a token-endpoint response we
// will buffer in memory. Real responses are well under 10 KB; the cap
// is loose enough that no real provider hits it but tight enough to
// protect against a hostile or misconfigured server streaming
// unbounded bytes.
const maxTokenResponseBytes = 1 << 20 // 1 MiB

// maxExpiresInSeconds clamps a server-supplied expires_in. ~68 years
// is well past any plausible token lifetime and keeps the resulting
// nanosecond duration comfortably inside int64.
const maxExpiresInSeconds = 1 << 31

// do executes the assembled token request — encode body, build request,
// dispatch via Client.HTTPClient (or the default), parse the response.
// All non-validation errors come back as *TokenError.
func (c *Client) do(ctx context.Context, values url.Values) (*Token, error) {
	var (
		contentType string
		body        []byte
		err         error
	)
	if c.BodyEncoder != nil {
		contentType, body, err = c.BodyEncoder(values)
		if err != nil {
			return nil, &TokenError{Err: fmt.Errorf("encoding body: %w", err)}
		}
	} else {
		contentType = "application/x-www-form-urlencoded"
		body = []byte(values.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, bytes.NewReader(body))
	if err != nil {
		return nil, &TokenError{Err: fmt.Errorf("building token request: %w", err)}
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	for k, vs := range c.Headers {
		// Content-Type is owned by the BodyEncoder; ignore any
		// caller-supplied value to avoid duplicate/conflicting headers.
		if http.CanonicalHeaderKey(k) == "Content-Type" {
			continue
		}
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = defaultHTTPClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, &TokenError{Err: err}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponseBytes))
	if err != nil {
		return nil, &TokenError{HTTPStatus: resp.StatusCode, Err: err}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		te := &TokenError{HTTPStatus: resp.StatusCode}
		var parsed struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
			ErrorURI         string `json:"error_uri"`
		}
		if jsonErr := json.Unmarshal(respBody, &parsed); jsonErr == nil && parsed.Error != "" {
			te.Code = parsed.Error
			te.Description = parsed.ErrorDescription
			te.URI = parsed.ErrorURI
		} else {
			te.Body = respBody
		}
		return nil, te
	}

	raw := map[string]any{}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, &TokenError{
			HTTPStatus: resp.StatusCode,
			Body:       respBody,
			Err:        fmt.Errorf("parsing token response: %w", err),
		}
	}

	tok := &Token{Raw: raw}
	if v, ok := raw["access_token"].(string); ok {
		tok.AccessToken = v
	}
	if v, ok := raw["token_type"].(string); ok {
		tok.TokenType = v
	}
	if v, ok := raw["refresh_token"].(string); ok {
		tok.RefreshToken = v
	}
	if v, ok := raw["scope"].(string); ok {
		tok.Scope = v
	}
	// expires_in: per RFC 6749 §5.1 it is a number of seconds. Most
	// servers send a JSON number (decoded to float64); some send a
	// string. Accept both. Clamp absurd values to maxExpiresInSeconds
	// so a hostile server returning e.g. 1e18 cannot overflow the
	// time.Duration multiplication and produce a past-dated ExpiresAt
	// (which would make Token.Expired() return true immediately and
	// can drive a refresh loop).
	switch v := raw["expires_in"].(type) {
	case float64:
		if v > 0 {
			if v > maxExpiresInSeconds {
				v = maxExpiresInSeconds
			}
			tok.ExpiresAt = time.Now().Add(time.Duration(v) * time.Second)
		}
	case string:
		// time.ParseDuration already errors on overflow (~292 y cap),
		// so no extra clamp is needed here.
		if secs, err := time.ParseDuration(v + "s"); err == nil && secs > 0 {
			tok.ExpiresAt = time.Now().Add(secs)
		}
	}
	return tok, nil
}
