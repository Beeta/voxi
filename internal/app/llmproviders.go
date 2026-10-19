package app

import (
	"fmt"
	"strings"
	"time"
)

// LLMProvider is one saved OpenAI-compatible endpoint and the models the user
// keeps for it.
type LLMProvider struct {
	ID      string `json:"id"` // stable: the name can be edited, the choice must not follow it
	Name    string `json:"name"`
	BaseURL string `json:"baseURL"`
	Key     string `json:"key"`
	// Models is the model field split at commas exactly as typed, spaces and
	// empty entries included, so the input round-trips while it is being
	// edited (a trailing comma would vanish otherwise); modelNames is what
	// the rest of the app uses.
	Models []string `json:"models"`
}

// LLMChoice is the provider × model the app talks to.
type LLMChoice struct {
	Provider string `json:"provider"` // LLMProvider.ID
	Model    string `json:"model"`
}

// modelNames is the provider's models trimmed, without empty entries or
// repeats. A full-width comma separates too, since it is what a Chinese
// input method types.
func (p LLMProvider) modelNames() []string {
	var out []string
	seen := map[string]bool{}
	for _, part := range p.Models {
		for _, m := range strings.Split(part, "，") {
			m = strings.TrimSpace(m)
			if m != "" && !seen[m] {
				seen[m] = true
				out = append(out, m)
			}
		}
	}
	return out
}

// splitModels and joinModels convert between the model field's text and
// LLMProvider.Models; joinModels(splitModels(s)) == s for any s.
func splitModels(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func joinModels(m []string) string { return strings.Join(m, ",") }

// llmOption is one entry of the model picker.
type llmOption struct {
	Provider LLMProvider
	Model    string
	Label    string // "供应商 / 模型", unique among the options
}

// llmOptions lists every provider × model in order. Labels are what the
// picker binds to, so two providers sharing a name get a numbered suffix
// instead of one choice silently standing in for the other.
func llmOptions(ps []LLMProvider) []llmOption {
	var out []llmOption
	used := map[string]int{}
	for _, p := range ps {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			name = "未命名"
		}
		for _, m := range p.modelNames() {
			label := name + " / " + m
			used[label]++
			if n := used[label]; n > 1 {
				label = fmt.Sprintf("%s (%d)", label, n)
			}
			out = append(out, llmOption{Provider: p, Model: m, Label: label})
		}
	}
	return out
}

// activeLLM is the picked provider × model, or the first option when the
// pick no longer exists (its provider deleted, its model removed), so
// deleting the provider in use falls to the next one rather than leaving the
// app unconfigured. ok is false when there is nothing to pick.
func activeLLM(s Settings) (llmOption, bool) {
	opts := llmOptions(s.LLMProviders)
	for _, o := range opts {
		if o.Provider.ID == s.LLMActive.Provider && o.Model == s.LLMActive.Model {
			return o, true
		}
	}
	if len(opts) == 0 {
		return llmOption{}, false
	}
	return opts[0], true
}

// provider finds a saved provider by id for editing in place.
func (s *Settings) provider(id string) *LLMProvider {
	for i := range s.LLMProviders {
		if s.LLMProviders[i].ID == id {
			return &s.LLMProviders[i]
		}
	}
	return nil
}

// removeProvider deletes a saved provider.
func (s *Settings) removeProvider(id string) {
	out := s.LLMProviders[:0]
	for _, p := range s.LLMProviders {
		if p.ID != id {
			out = append(out, p)
		}
	}
	s.LLMProviders = out
}

// newProviderID is a fresh LLMProvider.ID.
func newProviderID() string { return fmt.Sprintf("p-%d", time.Now().UnixNano()) }

// legacyProviderID is the provider migrated from the single endpoint that
// settings files before 0.3.0 hold.
const legacyProviderID = "p-legacy"

// migrateLLM turns the single endpoint of an older settings file into the
// first saved provider, picked, so upgrading keeps the model working.
func migrateLLM(s *Settings) {
	if len(s.LLMProviders) > 0 {
		return
	}
	if strings.TrimSpace(s.LLMBaseURL) == "" && strings.TrimSpace(s.LLMKey) == "" && strings.TrimSpace(s.LLMModel) == "" {
		return
	}
	s.LLMProviders = []LLMProvider{{
		ID: legacyProviderID, Name: "默认",
		BaseURL: s.LLMBaseURL, Key: s.LLMKey, Models: splitModels(strings.TrimSpace(s.LLMModel)),
	}}
	s.LLMActive = LLMChoice{Provider: legacyProviderID, Model: strings.TrimSpace(s.LLMModel)}
}

// mirrorLegacyLLM writes the active choice into the old single-endpoint
// fields, so an older build opened on this settings file still finds a
// working model.
func mirrorLegacyLLM(s *Settings) {
	o, ok := activeLLM(*s)
	if !ok {
		s.LLMBaseURL, s.LLMKey, s.LLMModel = "", "", ""
		return
	}
	s.LLMBaseURL, s.LLMKey, s.LLMModel = o.Provider.BaseURL, o.Provider.Key, o.Model
}
