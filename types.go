package pinoauth

import (
	"context"
)

// Credentials holds OAuth tokens for a provider.
//
// Credentials is a plain data carrier; concurrent use must be guarded by
// the caller.
type Credentials struct {
	// Refresh is the refresh token (may be empty for providers that
	// don't issue one).
	Refresh string `json:"refresh"`
	// Access is the bearer access token.
	Access string `json:"access"`
	// Expires is the access-token expiry as a Unix timestamp in
	// milliseconds.
	Expires int64 `json:"expires"`
	// Extra carries provider-specific fields that don't fit the common
	// shape (e.g. an ID token, account ID, scope set).
	Extra map[string]any `json:"extra,omitempty"`
}

// Prompt describes a text prompt shown to the user during login.
type Prompt struct {
	// Message is the prompt label shown to the user.
	Message string
	// Placeholder is suggested input text (UI hint only).
	Placeholder string
	// AllowEmpty permits the user to submit an empty response.
	AllowEmpty bool
}

// AuthInfo describes a URL the user should visit to authorize.
type AuthInfo struct {
	// URL is the authorization URL to open in a browser.
	URL string
	// Instructions is human-readable guidance shown alongside URL.
	Instructions string
}

// LoginCallbacks are UI hooks invoked during a [Provider] login flow.
// All fields are optional; a nil hook is a no-op (or returns "", nil for
// input hooks).
//
// Cancellation is conveyed via the ctx argument to [Provider.Login], not
// through this struct.
type LoginCallbacks struct {
	// OnAuth is called when the user should visit a URL to authorize.
	OnAuth func(info AuthInfo)
	// OnPrompt asks the user for text input (e.g. a code).
	OnPrompt func(prompt Prompt) (string, error)
	// OnProgress reports a status message during the flow.
	OnProgress func(message string)
	// OnManualCodeInput asks the user to paste an auth code manually.
	OnManualCodeInput func() (string, error)
	// OnDismissManualInput is called when the browser callback succeeds
	// and any visible manual-input prompt should be hidden.
	OnDismissManualInput func()
}

// Provider is the interface that an OAuth login implementation satisfies.
//
// pinoauth itself ships no concrete providers; Provider exists as a
// shared shape so callers can plug different login flows behind one type.
type Provider interface {
	// ID returns a stable provider identifier (e.g. "anthropic").
	ID() string
	// Name returns a human-readable provider name.
	Name() string
	// Login runs the full OAuth login flow and returns credentials to
	// persist. Implementations must honour ctx for cancellation.
	Login(ctx context.Context, callbacks LoginCallbacks) (*Credentials, error)
	// UsesCallbackServer reports whether Login uses a loopback HTTP
	// callback server (and thus supports manual-code-input fallback).
	UsesCallbackServer() bool
	// RefreshToken exchanges expired credentials for fresh ones.
	// Implementations must honour ctx for cancellation.
	RefreshToken(ctx context.Context, creds *Credentials) (*Credentials, error)
	// GetAPIKey extracts the API key string from credentials, or "" if
	// the provider does not expose one.
	GetAPIKey(creds *Credentials) string
	// ListModels returns the model IDs available for the given
	// credentials. It returns nil, nil when live listing is not
	// supported (callers should fall back to permissive mode).
	ListModels(ctx context.Context, creds *Credentials) ([]string, error)
}

// ProviderInfo describes an OAuth provider for display in the UI.
type ProviderInfo struct {
	// ID is the stable provider identifier.
	ID string
	// Name is the human-readable provider name.
	Name string
	// Available reports whether the provider is currently usable
	// (e.g. config loaded, dependencies present).
	Available bool
}
