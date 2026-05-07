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
func (p *testProvider) Login(_ context.Context, _ LoginCallbacks) (*Credentials, error) {
	return &Credentials{Access: "test-token"}, nil
}
func (p *testProvider) RefreshToken(_ context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}
func (p *testProvider) GetAPIKey(creds *Credentials) string {
	return creds.Access
}
func (p *testProvider) ListModels(_ context.Context, _ *Credentials) ([]string, error) {
	return nil, nil
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

	creds, err := p.Login(context.Background(), LoginCallbacks{})
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	if creds.Access != "test-token" {
		t.Errorf("expected access 'test-token', got %q", creds.Access)
	}

	refreshed, err := p.RefreshToken(context.Background(), creds)
	if err != nil {
		t.Fatalf("RefreshToken error: %v", err)
	}
	if refreshed.Access != creds.Access {
		t.Errorf("RefreshToken changed Access: got %q", refreshed.Access)
	}

	key := p.GetAPIKey(creds)
	if key != "test-token" {
		t.Errorf("expected key 'test-token', got %q", key)
	}
}

func TestCredentials_ExtraFields(t *testing.T) {
	c := Credentials{
		Refresh: "r",
		Access:  "a",
		Expires: 1234567890,
		Extra: map[string]any{
			"scope":  "read write",
			"custom": 42.0,
		},
	}
	if c.Extra["scope"] != "read write" {
		t.Error("expected scope in extra")
	}
	if c.Extra["custom"] != 42.0 {
		t.Error("expected custom in extra")
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
