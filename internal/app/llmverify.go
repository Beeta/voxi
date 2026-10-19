package app

import (
	"slices"
	"strings"

	"github.com/Beeta/voxi/internal/llm"
)

// verifyLLM checks one saved provider in the background by sending one
// minimal chat request with its first model, then shows the result under
// that provider in the settings dialog. It verifies the provider being
// edited rather than the one in use, so a newly added one can be checked
// before it is picked.
func (a *app) verifyLLM(id string) {
	s := a.settings.snapshot()
	p := s.provider(id)
	if p == nil {
		return
	}
	base := strings.TrimSpace(p.BaseURL)
	key := strings.TrimSpace(p.Key)
	var model string
	if ms := p.modelNames(); len(ms) > 0 {
		model = ms[0]
	}
	a.mu.Lock()
	if a.verifyingLLM {
		a.mu.Unlock()
		return
	}
	a.llmVerifyID = id
	if base == "" || key == "" || model == "" {
		a.llmVerifyOK = false
		a.llmVerifyMsg = "请先填写 Base URL、API Key 和模型。"
		a.mu.Unlock()
		a.ping()
		return
	}
	a.verifyingLLM = true
	a.llmVerifyMsg = ""
	a.mu.Unlock()
	a.ping()
	go func() {
		err := llm.Verify(base, key, model)
		a.mu.Lock()
		a.verifyingLLM = false
		if err != nil {
			a.llmVerifyOK, a.llmVerifyMsg = false, "连接失败（"+model+"）："+err.Error()
		} else {
			a.llmVerifyOK, a.llmVerifyMsg = true, "连接正常（"+model+"）"
		}
		a.mu.Unlock()
		a.ping()
	}()
}

// fetchModels asks a saved provider which models it serves, in the
// background, and keeps the list for its card to offer.
func (a *app) fetchModels(id string) {
	s := a.settings.snapshot()
	p := s.provider(id)
	if p == nil {
		return
	}
	base, key := strings.TrimSpace(p.BaseURL), strings.TrimSpace(p.Key)
	a.mu.Lock()
	if a.llmFetching[id] {
		a.mu.Unlock()
		return
	}
	if base == "" || key == "" {
		a.llmFetchErr[id] = "请先填写 Base URL 和 API Key。"
		a.mu.Unlock()
		a.ping()
		return
	}
	a.llmFetching[id] = true
	delete(a.llmFetchErr, id)
	a.mu.Unlock()
	a.ping()
	go func() {
		ids, err := llm.ListModels(base, key)
		a.mu.Lock()
		delete(a.llmFetching, id)
		if err != nil {
			a.llmFetchErr[id] = "获取失败：" + err.Error()
		} else {
			a.llmFetched[id] = ids
		}
		a.mu.Unlock()
		a.ping()
	}()
}

// toggleModel adds model to a provider's list, or takes it out when it is
// there already. The list is rewritten clean (trimmed, deduplicated).
func (a *app) toggleModel(id, model string) {
	a.settings.update(func(st *Settings) {
		p := st.provider(id)
		if p == nil {
			return
		}
		names := p.modelNames()
		if i := slices.Index(names, model); i >= 0 {
			names = slices.Delete(names, i, i+1)
		} else {
			names = append(names, model)
		}
		p.Models = names
	})
}
