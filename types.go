package pinoauth

import (
	"context"
)

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
	// ShortURL is an optional pre-shortened form of URL produced by
	// the provider (e.g. via a public URL shortener that forwards
	// click-time query params). Callers typically present it
	// prominently and fall back to URL. Empty means no short form.
	ShortURL string
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

// Provider is a convention for assembling pinoauth's primitives into a
// provider-specific login flow. pinoauth itself ships no concrete
// providers; Provider exists as a shared shape so callers can plug
// different login flows behind one type.
//
// The interface is deliberately minimal: ID/Name for display, Login for
// the interactive flow, RefreshToken for renewal. Anything provider-
// specific (API-key extraction, model listing, account IDs) belongs in
// the concrete type's own methods, not here.
type Provider interface {
	// ID returns a stable provider identifier (e.g. "anthropic").
	ID() string
	// Name returns a human-readable provider name.
	Name() string
	// Login runs the full OAuth login flow and returns a token to
	// persist. Implementations must honour ctx for cancellation.
	Login(ctx context.Context, callbacks LoginCallbacks) (*Token, error)
	// UsesCallbackServer reports whether Login uses a loopback HTTP
	// callback server (and thus supports manual-code-input fallback).
	UsesCallbackServer() bool
	// RefreshToken exchanges an expired token for a fresh one.
	// Implementations must honour ctx for cancellation.
	RefreshToken(ctx context.Context, tok *Token) (*Token, error)
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
