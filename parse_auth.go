package pinoauth

import (
	"net/url"
	"strings"
)

// ParseAuthorizationInput extracts code and state from various input formats:
//   - full callback URL (state is taken from the query string)
//   - code#state (the OpenAI-style "manual entry" form)
//   - query-string fragment containing code=…&state=…
//   - a bare code (state empty)
//
// Common shell-escape backslashes pasted from terminal output are stripped.
func ParseAuthorizationInput(input string) (code, state string) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", ""
	}

	// Strip shell-escape backslashes (common when pasting from terminal output).
	value = strings.ReplaceAll(value, "\\", "")

	// Try URL
	if u, err := url.Parse(value); err == nil && u.Scheme != "" {
		return u.Query().Get("code"), u.Query().Get("state")
	}

	// Try code#state
	if strings.Contains(value, "#") {
		parts := strings.SplitN(value, "#", 2)
		return parts[0], parts[1]
	}

	// Try query-string format
	if strings.Contains(value, "code=") {
		params, _ := url.ParseQuery(value)
		return params.Get("code"), params.Get("state")
	}

	// Bare code
	return value, ""
}
