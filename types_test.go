package pinoauth

import (
	"context"
	"testing"
)

// testProvider is a minimal Provider implementation for testing.
type testProvider struct {
	id   string
	name string
}

func (p *testProvider) ID() string               { return p.id }
func (p *testProvider) Name() string             { return p.name }
func (p *testProvider) UsesCallbackServer() bool { return false }
func (p *testProvider) Login(_ context.Context, _ LoginCallbacks) (*Token, error) {
	return &Token{AccessToken: "test-token"}, nil
}
func (p *testProvider) RefreshToken(_ context.Context, tok *Token) (*Token, error) {
	return tok, nil
}

func TestProviderInterface(t *testing.T) {
	var p Provider = &testProvider{id: "test", name: "Test Provider"}

	if p.ID() != "test" {
		t.Errorf("expected ID 'test', got %q", p.ID())
	}
	if p.Name() != "Test Provider" {
		t.Errorf("expected Name 'Test Provider', got %q", p.Name())
	}
	if p.UsesCallbackServer() {
		t.Error("expected UsesCallbackServer() == false")
	}

	tok, err := p.Login(context.Background(), LoginCallbacks{})
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	if tok.AccessToken != "test-token" {
		t.Errorf("expected access 'test-token', got %q", tok.AccessToken)
	}

	refreshed, err := p.RefreshToken(context.Background(), tok)
	if err != nil {
		t.Fatalf("RefreshToken error: %v", err)
	}
	if refreshed.AccessToken != tok.AccessToken {
		t.Errorf("RefreshToken changed AccessToken: got %q", refreshed.AccessToken)
	}
}

func TestProviderInfo(t *testing.T) {
	info := ProviderInfo{
		ID:        "anthropic",
		Name:      "Anthropic",
		Available: true,
	}
	if info.ID != "anthropic" {
		t.Error("wrong ID")
	}
	if !info.Available {
		t.Error("expected available")
	}
}
