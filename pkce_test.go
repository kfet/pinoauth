package pinoauth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestGeneratePKCE_ReturnsValidChallenge(t *testing.T) {
	p := GeneratePKCE()
	if p.Verifier == "" {
		t.Error("Verifier should not be empty")
	}
	if p.Challenge == "" {
		t.Error("Challenge should not be empty")
	}
}

func TestGeneratePKCE_VerifierLength(t *testing.T) {
	p := GeneratePKCE()
	// 32 bytes base64url-encoded = 43 chars (no padding)
	if len(p.Verifier) != 43 {
		t.Errorf("Verifier length = %d, want 43", len(p.Verifier))
	}
}

func TestGeneratePKCE_ChallengeLength(t *testing.T) {
	p := GeneratePKCE()
	// SHA-256 = 32 bytes, base64url-encoded = 43 chars (no padding)
	if len(p.Challenge) != 43 {
		t.Errorf("Challenge length = %d, want 43", len(p.Challenge))
	}
}

func TestGeneratePKCE_ChallengeMatchesVerifier(t *testing.T) {
	p := GeneratePKCE()

	// Manually compute SHA-256 of verifier and compare
	hash := sha256.Sum256([]byte(p.Verifier))
	expected := base64.RawURLEncoding.EncodeToString(hash[:])

	if p.Challenge != expected {
		t.Errorf("Challenge = %q, expected SHA-256(verifier) = %q", p.Challenge, expected)
	}
}

func TestGeneratePKCE_VerifierAndChallengeAreDifferent(t *testing.T) {
	p := GeneratePKCE()
	if p.Verifier == p.Challenge {
		t.Error("Verifier and Challenge should be different")
	}
}

func TestGeneratePKCE_TwoCallsProduceDifferentValues(t *testing.T) {
	p1 := GeneratePKCE()
	p2 := GeneratePKCE()
	if p1.Verifier == p2.Verifier {
		t.Error("Two calls should produce different verifiers")
	}
	if p1.Challenge == p2.Challenge {
		t.Error("Two calls should produce different challenges")
	}
}

func TestGenerateState_Length(t *testing.T) {
	s := GenerateState()
	// 32 bytes base64url-encoded = 43 chars (no padding)
	if len(s) != 43 {
		t.Errorf("GenerateState length = %d, want 43", len(s))
	}
}

func TestGenerateState_TwoCallsProduceDifferentValues(t *testing.T) {
	s1 := GenerateState()
	s2 := GenerateState()
	if s1 == s2 {
		t.Error("Two calls should produce different state values")
	}
}

func TestBase64URLEncode_NoPadding(t *testing.T) {
	// 1 byte encodes to 2 chars with padding "AQ==", but base64url raw has no padding
	result := base64URLEncode([]byte{1})
	if result != "AQ" {
		t.Errorf("base64URLEncode([1]) = %q, want %q", result, "AQ")
	}
}

func TestBase64URLEncode_URLSafe(t *testing.T) {
	// Bytes that would produce + and / in standard base64
	// 0xfb, 0xff, 0xfe → standard base64 = "+//+" → base64url = "-__-"
	result := base64URLEncode([]byte{0xfb, 0xff, 0xfe})
	if result != "-__-" {
		t.Errorf("base64URLEncode = %q, want %q (url-safe)", result, "-__-")
	}
}
