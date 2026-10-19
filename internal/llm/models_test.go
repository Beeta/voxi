package llm

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// TestModelsEndpoint keeps the model list beside the chat endpoint however
// the base URL was typed.
func TestModelsEndpoint(t *testing.T) {
	for base, want := range map[string]string{
		"https://api.deepseek.com":                     "https://api.deepseek.com/models",
		"https://api.openai.com/v1/":                   "https://api.openai.com/v1/models",
		"https://x.example/v1/chat/completions":        "https://x.example/v1/models",
		" https://x.example/api/v3/chat/completions/ ": "https://x.example/api/v3/models",
	} {
		if got := ModelsEndpoint(base); got != want {
			t.Errorf("ModelsEndpoint(%q) = %q, want %q", base, got, want)
		}
	}
}

// TestListModels reads the ids out of an OpenAI-shaped reply, sorted, sends
// the key, and turns a refusal into an error carrying the status.
func TestListModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer good" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"bad key"}`))
			return
		}
		w.Write([]byte(`{"object":"list","data":[{"id":"gpt-b"},{"id":" "},{"id":"gpt-a"}]}`))
	}))
	defer srv.Close()

	ids, err := ListModels(srv.URL+"/v1", "good")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, []string{"gpt-a", "gpt-b"}) {
		t.Errorf("ids %q", ids)
	}
	if _, err := ListModels(srv.URL+"/v1", "bad"); err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("a refused key gave %v", err)
	}
	if _, err := ListModels(srv.URL, "good"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("a wrong path gave %v", err)
	}
}
