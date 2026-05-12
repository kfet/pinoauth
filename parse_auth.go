package pinoauth

import (
	"net/url"
	"strings"
)

// ParseAuthorizationInput extracts the authorization code and state from
// whatever the user pastes back from a browser. It accepts:
//
//   - a full callback URL (state is read from the query string);
//   - the "code#state" form (used by some providers' manual-entry pages);
//   - a bare query-string fragment containing code=…&state=…;
//   - a bare authorization code (state will be empty).
//
// Shell-escape backslashes pasted from terminal output are removed when the
// input shows shell-escape "tells" (e.g. \?, \&, \=); backslashes elsewhere
// are preserved, since RFC 6749 §A.11 allows '\' inside VSCHAR codes.
// ParseAuthorizationInput does no validation beyond extraction; callers
// must compare state against their expected value.
func ParseAuthorizationInput(input string) (code, state string) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", ""
	}

	// If the input looks shell-escaped (\? \& \= \# "\ "), unescape it as a
	// shell would: \X -> X (so \\ -> \). Otherwise leave backslashes alone.
	if hasShellEscapeTell(value) {
		value = shellUnescape(value)
	}

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

// hasShellEscapeTell reports whether s contains a backslash immediately
// followed by a URL-structural character that a shell would have escaped
// when emitting the URL as terminal output (?, &, =, #, or space). The
// presence of any such pair is unambiguous evidence the whole string was
// shell-escaped, since these characters never appear after a literal '\'
// in valid URL syntax.
func hasShellEscapeTell(s string) bool {
	for i := 0; i < len(s)-1; i++ {
		if s[i] != '\\' {
			continue
		}
		switch s[i+1] {
		case '?', '&', '=', '#', ' ':
			return true
		}
	}
	return false
}

// shellUnescape walks s treating '\' as "take the next byte literally", so
// "\\" -> "\", "\?" -> "?", etc. A trailing lone '\' is dropped.
func shellUnescape(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			b.WriteByte(s[i+1])
			i++
			continue
		}
		if s[i] == '\\' {
			// trailing backslash with nothing to escape; drop it
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
