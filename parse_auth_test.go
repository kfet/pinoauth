package pinoauth

import "testing"

func TestParseAuthorizationInput(t *testing.T) {
	tests := []struct {
		input     string
		wantCode  string
		wantState string
	}{
		// URL format
		{
			"http://localhost:1455/auth/callback?code=abc123&state=xyz",
			"abc123", "xyz",
		},
		// code#state format
		{"mycode#mystate", "mycode", "mystate"},
		// URL params format
		{"code=abc&state=def", "abc", "def"},
		// Raw code
		{"justcode", "justcode", ""},
		// Empty
		{"", "", ""},
		{"  ", "", ""},
		// Shell-escaped URL (backslashes from terminal copy-paste)
		{
			`http://localhost:1455/auth/callback\?code\=ac_abc\&state\=xyz`,
			"ac_abc", "xyz",
		},
	}
	for _, tt := range tests {
		code, state := ParseAuthorizationInput(tt.input)
		if code != tt.wantCode {
			t.Errorf("ParseAuthorizationInput(%q) code = %q, want %q", tt.input, code, tt.wantCode)
		}
		if state != tt.wantState {
			t.Errorf("ParseAuthorizationInput(%q) state = %q, want %q", tt.input, state, tt.wantState)
		}
	}
}
