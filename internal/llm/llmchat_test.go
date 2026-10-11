package llm

import "testing"

// TestEndpoint pins the URL the settings hint shows and the client requests:
// the trailing slash goes away, /chat/completions is appended unless it is
// already there, and the /v1 segment is left exactly as the user typed it
// (DeepSeek accepts it either way, OpenAI requires it).
func TestEndpoint(t *testing.T) {
	cases := []struct{ base, want string }{
		{"https://api.deepseek.com", "https://api.deepseek.com/chat/completions"},
		{"https://api.deepseek.com/", "https://api.deepseek.com/chat/completions"},
		{"https://api.deepseek.com/v1", "https://api.deepseek.com/v1/chat/completions"},
		{"https://api.deepseek.com/v1/", "https://api.deepseek.com/v1/chat/completions"},
		{"  https://api.openai.com/v1  ", "https://api.openai.com/v1/chat/completions"},
		{"https://example.com/v1/chat/completions", "https://example.com/v1/chat/completions"},
		{"", "/chat/completions"},
	}
	for _, tc := range cases {
		if got := Endpoint(tc.base); got != tc.want {
			t.Errorf("Endpoint(%q) = %q, want %q", tc.base, got, tc.want)
		}
	}
}
