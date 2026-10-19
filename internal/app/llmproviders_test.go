package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestMigrateLLM turns the single endpoint of a settings file from before
// 0.3.0 into a picked provider, so upgrading keeps summaries working.
func TestMigrateLLM(t *testing.T) {
	s := Settings{LLMBaseURL: "https://api.example.com/v1", LLMKey: "k", LLMModel: " gpt-x "}
	migrateLLM(&s)
	o, ok := activeLLM(s)
	if !ok || o.Provider.BaseURL != "https://api.example.com/v1" || o.Provider.Key != "k" || o.Model != "gpt-x" {
		t.Fatalf("migrated to %+v (ok=%v)", o, ok)
	}
	// A second run, or a file that already has providers, leaves them be.
	s.LLMProviders[0].Name = "改过的名字"
	migrateLLM(&s)
	if len(s.LLMProviders) != 1 || s.LLMProviders[0].Name != "改过的名字" {
		t.Errorf("migrating again rewrote the providers: %+v", s.LLMProviders)
	}

	var empty Settings
	migrateLLM(&empty)
	if len(empty.LLMProviders) != 0 {
		t.Errorf("an unconfigured file grew a provider: %+v", empty.LLMProviders)
	}
}

// TestOldSettingsFileUpgrades runs the migration through the real load and
// save: the old fields still mirror the choice afterwards, for an older
// build opened on the same file.
func TestOldSettingsFileUpgrades(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	os.WriteFile(path, []byte(`{"llmBaseURL":"https://a/v1","llmKey":"k","llmModel":"m1"}`), 0o644)
	var s Settings
	data, _ := os.ReadFile(path)
	json.Unmarshal(data, &s)
	migrateLLM(&s)
	st := &settingsStore{path: path, s: &s}
	st.update(func(s *Settings) {
		s.LLMProviders = append(s.LLMProviders, LLMProvider{ID: "p2", Name: "B", BaseURL: "https://b/v1", Key: "kb", Models: []string{"m2"}})
		s.LLMActive = LLMChoice{Provider: "p2", Model: "m2"}
	})
	data, _ = os.ReadFile(path)
	var back Settings
	json.Unmarshal(data, &back)
	if back.LLMBaseURL != "https://b/v1" || back.LLMKey != "kb" || back.LLMModel != "m2" {
		t.Errorf("the old fields hold %q %q %q, want the active choice", back.LLMBaseURL, back.LLMKey, back.LLMModel)
	}
	if len(back.LLMProviders) != 2 {
		t.Errorf("saved %d providers, want 2", len(back.LLMProviders))
	}
}

// TestLLMOptions pins the picker's entries: every provider × model in order,
// models trimmed and deduplicated across both kinds of comma, and two
// providers of the same name told apart rather than one shadowing the other.
func TestLLMOptions(t *testing.T) {
	ps := []LLMProvider{
		{ID: "a", Name: "DeepSeek", Models: splitModels("chat, reasoner，chat,,")},
		{ID: "b", Name: "DeepSeek", Models: []string{"chat"}},
		{ID: "c", Name: "", Models: []string{"x"}},
		{ID: "d", Name: "没模型"},
	}
	var got []string
	for _, o := range llmOptions(ps) {
		got = append(got, o.Label)
	}
	want := []string{"DeepSeek / chat", "DeepSeek / reasoner", "DeepSeek / chat (2)", "未命名 / x"}
	if !slices.Equal(got, want) {
		t.Errorf("options %q, want %q", got, want)
	}
}

// TestActiveLLMFallsBack: once the picked provider is gone the app uses the
// first remaining option instead of reporting the model unconfigured.
func TestActiveLLMFallsBack(t *testing.T) {
	s := Settings{
		LLMProviders: []LLMProvider{{ID: "a", Name: "A", Models: []string{"m1", "m2"}}, {ID: "b", Name: "B", Models: []string{"n"}}},
		LLMActive:    LLMChoice{Provider: "a", Model: "m2"},
	}
	if o, _ := activeLLM(s); o.Provider.ID != "a" || o.Model != "m2" {
		t.Errorf("active is %s/%s, want a/m2", o.Provider.ID, o.Model)
	}
	s.removeProvider("a")
	if o, ok := activeLLM(s); !ok || o.Provider.ID != "b" || o.Model != "n" {
		t.Errorf("after deleting the active provider: %s/%s ok=%v, want b/n", o.Provider.ID, o.Model, ok)
	}
	s.removeProvider("b")
	if _, ok := activeLLM(s); ok {
		t.Error("with no providers left there is still an active model")
	}
}

// TestModelFieldRoundTrips: the field is saved on every keystroke, so a
// trailing comma or space typed on the way to the next model must survive.
func TestModelFieldRoundTrips(t *testing.T) {
	for _, s := range []string{"", "a", "a,", "a, ", "a, b", ",,"} {
		if got := joinModels(splitModels(s)); got != s {
			t.Errorf("%q came back as %q", s, got)
		}
	}
}

// TestPickingAModelSavesIt chooses the second model through the real
// drop-down and wants it saved. A select writes the choice at the end of the
// pass, so only a real click shows whether the section reads it through
// Changed.
func TestPickingAModelSavesIt(t *testing.T) {
	a, set := newSettingsApp(t)
	set.update(func(s *Settings) {
		s.LLMProviders = []LLMProvider{
			{ID: "a", Name: "甲", BaseURL: "https://a/v1", Key: "k", Models: []string{"m1"}},
			{ID: "b", Name: "乙", BaseURL: "https://b/v1", Key: "k", Models: []string{"m2"}},
		}
	})
	tt := ui.NewTester(func(c *ui.Context) { a.llmPicker(c, set.snapshot()) }, 480, 400)
	if err := tt.Click("甲 / m1"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("乙 / m2"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := set.snapshot().LLMActive; got != (LLMChoice{Provider: "b", Model: "m2"}) {
		t.Errorf("picking 乙 / m2 saved %+v", got)
	}
	if _, ok := a.llm(); !ok {
		t.Error("the picked model gives no client")
	}
}

// TestAddAndDeleteProvider drives the section's buttons: 添加 appends an
// editable provider, and 删除 goes through the confirmation.
func TestAddAndDeleteProvider(t *testing.T) {
	a, set := newSettingsApp(t)
	tt := ui.NewTester(func(c *ui.Context) { a.llmSettings(c, set.snapshot()) }, 616, 900)
	if err := tt.Click("+ 添加供应商"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if n := len(set.snapshot().LLMProviders); n != 1 {
		t.Fatalf("after 添加 there are %d providers", n)
	}
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("删除供应商"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if n := len(set.snapshot().LLMProviders); n != 0 {
		t.Errorf("after confirming the delete there are %d providers", n)
	}
}

// TestFetchedModelsToggle fetches from a fake /models, then clicks a chip to
// add the model and again to take it out, through the real card.
func TestFetchedModelsToggle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"alpha"},{"id":"beta"}]}`))
	}))
	defer srv.Close()
	a, set := newSettingsApp(t)
	set.update(func(s *Settings) {
		s.LLMProviders = []LLMProvider{{ID: "a", Name: "甲", BaseURL: srv.URL, Key: "k", Models: []string{"alpha"}}}
	})
	tt := ui.NewTester(func(c *ui.Context) { a.llmSettings(c, set.snapshot()) }, 616, 1200)
	if err := tt.Click("获取模型列表"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200 && !tt.HasText("beta"); i++ {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText("beta") {
		t.Fatalf("the fetched models did not show: %q", tt.Texts())
	}
	if err := tt.Click("beta"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := set.snapshot().LLMProviders[0].modelNames(); !slices.Equal(got, []string{"alpha", "beta"}) {
		t.Fatalf("after clicking beta the models are %q", got)
	}
	if err := tt.Click("beta"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := set.snapshot().LLMProviders[0].modelNames(); !slices.Equal(got, []string{"alpha"}) {
		t.Errorf("clicking beta again left %q", got)
	}
}

// TestFilterModels: every word must appear, in any case.
func TestFilterModels(t *testing.T) {
	ids := []string{"gpt-4o", "gpt-4o-mini", "DeepSeek-Chat", "o1"}
	if got := filterModels(ids, "4O mini"); !slices.Equal(got, []string{"gpt-4o-mini"}) {
		t.Errorf("4O mini matched %q", got)
	}
	if got := filterModels(ids, " "); !slices.Equal(got, ids) {
		t.Errorf("an empty filter matched %q", got)
	}
}

// TestQuickModelSelect: the tabs only offer a picker when there is a choice
// to make, and picking there is the same saved choice as in settings.
func TestQuickModelSelect(t *testing.T) {
	a, set := newSettingsApp(t)
	set.update(func(s *Settings) {
		s.LLMProviders = []LLMProvider{{ID: "a", Name: "甲", BaseURL: "https://a", Key: "k", Models: []string{"m1"}}}
	})
	tt := ui.NewTester(func(c *ui.Context) { a.quickModelSelect(c, "q") }, 400, 300)
	tt.Frame()
	if tt.HasText("甲 / m1") {
		t.Error("a single model still gets a picker")
	}
	set.update(func(s *Settings) { s.LLMProviders[0].Models = []string{"m1", "m2"} })
	tt.Frame()
	if err := tt.Click("甲 / m1"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("甲 / m2"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := set.snapshot().LLMActive; got != (LLMChoice{Provider: "a", Model: "m2"}) {
		t.Errorf("picking in the tab saved %+v", got)
	}
}
