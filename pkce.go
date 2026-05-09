package pinoauth

import (
	"crypto/sha256"
	"encoding/base64"
)

// PKCEChallenge holds an RFC 7636 PKCE code verifier and its derived
// SHA-256 challenge. The zero value is not useful — obtain values via
// [GeneratePKCE].
type PKCEChallenge struct {
	// Verifier is the high-entropy code verifier (RFC 7636 §4.1),
	// base64url-encoded with no padding. Send to the token endpoint
	// during the code exchange.
	Verifier string
	// Challenge is BASE64URL(SHA256(Verifier)) (RFC 7636 §4.2). Send
	// to the authorization endpoint as code_challenge with
	// code_challenge_method=S256.
	Challenge string
}

// GeneratePKCE returns a fresh PKCE pair: a 32-byte cryptographically
// random verifier (base64url-encoded, no padding) and its S256 challenge.
//
// Callers should generate a new pair per authorization request.
//
// GeneratePKCE panics only if the kernel CSPRNG is unavailable, which
// on every supported platform indicates a host that cannot meaningfully
// continue.
func GeneratePKCE() *PKCEChallenge {
	verifierBytes := make([]byte, 32)
	mustReadRandom(verifierBytes)
	verifier := base64URLEncode(verifierBytes)

	hash := sha256.Sum256([]byte(verifier))
	challenge := base64URLEncode(hash[:])

	return &PKCEChallenge{
		Verifier:  verifier,
		Challenge: challenge,
	}
}

// GenerateState returns a 32-byte cryptographically random value,
// base64url-encoded with no padding, suitable for use as the OAuth 2.0
// "state" parameter (RFC 6749 §10.12).
//
// Callers should generate a new state per authorization request and
// compare it byte-for-byte against the value echoed back by the
// authorization server (or returned by [StartCallbackServer], which
// already enforces the comparison when given a non-empty expectedState).
//
// GenerateState panics only if the kernel CSPRNG is unavailable, which
// on every supported platform indicates a host that cannot meaningfully
// continue.
func GenerateState() string {
	b := make([]byte, 32)
	mustReadRandom(b)
	return base64URLEncode(b)
}

// base64URLEncode encodes bytes as a base64url string without padding.
func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}
