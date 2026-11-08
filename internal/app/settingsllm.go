package app

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/Beeta/voxi/internal/llm"
	"github.com/egoist/mygo/ui"
)

// endpointHint shows the URL the LLM client will actually request for the
// configured base URL, so a missing or extra /v1 is visible here rather than
// as a bare 404 from 验证连接.
func (a *app) endpointHint(c *ui.Context, base string) {
	if strings.TrimSpace(base) == "" {
		ui.Text(c, "填好 Base URL 后，请求地址会自动补上 /chat/completions。").FontSize(12).TextColor(muted)
		return
	}
	ui.Text(c, "请求地址："+llm.Endpoint(base)).FontSize(12).TextColor(muted)
}

// llmSettings picks the model in use among the saved providers × models, and
// edits the providers: add, edit, verify, delete.
//
// Each provider is one summary line until it is opened for editing, one at a
// time, so the page stays short however many there are.
func (a *app) llmSettings(c *ui.Context, s Settings) {
	settingsGroup(c, "set-llm-use", "", func() {
		a.llmPicker(c, s)
	}, func() {
		ui.Text(c, "摘要、脑图和问答都用这里选中的模型；各页底部也能临时切换。").FontSize(11.5).TextColor(muted)
	})
	settingsGroup(c, "set-llm-prov", "供应商", func() {
		for _, p := range s.LLMProviders {
			if p.ID == a.llmOpenProv {
				a.providerCard(c, p)
			} else {
				a.providerRow(c, p)
			}
		}
		add := ui.ButtonBase(c.Key("llm-add")).Padding(0, 14).MinHeight(40).Gap(6).AlignItems(ui.Center).Label("+ 添加供应商")
		add.Background(add.AnimateColor("bg", controlFill(add, ui.Transparent, hover), hoverFade))
		add.Children(func() {
			ui.Icon(c, iconPlus).FontSize(13).TextColor(accent)
			ui.Text(c, "添加供应商").FontSize(13).FontWeight(500).TextColor(accent)
		})
		if add.Clicked() {
			id := newProviderID()
			a.settings.update(func(st *Settings) {
				st.LLMProviders = append(st.LLMProviders, LLMProvider{ID: id, Name: "新供应商"})
			})
			a.llmOpenProv = id
		}
	}, nil)
	name := ""
	if p := s.provider(a.delProviderID); p != nil {
		name = p.Name
	}
	if i := ui.AlertDialog(c.Key("llm-delq"), &a.confirmDelProv, "删除供应商？",
		"将删除「"+name+"」及其 API Key 和模型；正在使用它时会改用列表里的第一个模型。", "删除供应商", "取消"); i == 0 {
		id := a.delProviderID
		a.settings.update(func(st *Settings) { st.removeProvider(id) })
	}
}

// llmPicker chooses the provider × model that summaries, mind maps and Q&A
// use.
func (a *app) llmPicker(c *ui.Context, s Settings) {
	settingsRow(c, "llm-pick", "使用模型", "", func() {
		if !a.modelSelect(c, "llm-select", s) {
			ui.Text(c, "先添加供应商并填写模型").FontSize(12).TextColor(muted)
		}
	})
}

// modelSelect is the drop-down of saved provider × model pairs, saving the
// pick; it draws nothing and reports false when there is nothing to pick.
// The settings dialog and the summary, mind-map and Q&A tabs share it, so
// switching in one shows in all.
func (a *app) modelSelect(c *ui.Context, key string, s Settings) bool {
	opts := llmOptions(s.LLMProviders)
	cur, ok := activeLLM(s)
	if !ok {
		return false
	}
	labels := make([]string, len(opts))
	chosen := ""
	for i, o := range opts {
		labels[i] = o.Label
		if o.Provider.ID == cur.Provider.ID && o.Model == cur.Model {
			chosen = o.Label
		}
	}
	// As with the other value-bound controls, the pick lands at the end of
	// the pass: only Changed sees it.
	if ui.Select(c.Key(key), &chosen, labels).Label("使用模型").FontSize(12).Changed() {
		for _, o := range opts {
			if o.Label == chosen {
				pick := LLMChoice{Provider: o.Provider.ID, Model: o.Model}
				a.settings.update(func(st *Settings) { st.LLMActive = pick })
			}
		}
	}
	return true
}

// quickModelSelect is modelSelect beside a tab's generate or send button:
// shown only when there is more than one model to choose from, and kept
// narrow so a long label ends in an ellipsis instead of squeezing the bar.
func (a *app) quickModelSelect(c *ui.Context, key string) {
	s := a.settings.snapshot()
	if len(llmOptions(s.LLMProviders)) < 2 {
		return
	}
	ui.Row(c.Key(key)).MaxWidth(220).Shrink(1).Children(func() {
		a.modelSelect(c, key+"-sel", s)
	})
}

// providerRow is a provider folded to one line: its name, how many models
// it has, the host it talks to and the last verification, with 编辑 to open
// it. A click anywhere on the line opens it too.
func (a *app) providerRow(c *ui.Context, p LLMProvider) {
	a.mu.Lock()
	var msg string
	var ok bool
	if a.llmVerifyID == p.ID {
		msg, ok = a.llmVerifyMsg, a.llmVerifyOK
	}
	a.mu.Unlock()
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = "未命名"
	}
	detail := strconv.Itoa(len(p.modelNames())) + " 个模型"
	if h := baseHost(p.BaseURL); h != "" {
		detail += " · " + h
	} else {
		detail += " · 未填 Base URL"
	}
	row := ui.ButtonBase(c.Key("llm-"+p.ID+"-row")).Padding(10, 14).MinHeight(52).Label(name)
	row.Background(row.AnimateColor("bg", controlFill(row, ui.Transparent, hover), hoverFade))
	// A button centers what it holds; the line fills it instead, the name
	// at the start and the chevron at the end.
	row.Children(func() {
		ui.Row(c).Grow(1).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, name).FontSize(13).FontWeight(500).TextColor(ink).SingleLine()
				ui.Text(c, detail).FontSize(12).TextColor(muted).SingleLine()
			})
			if msg != "" {
				if ok {
					statusMark(c, iconCheck, "连接正常", okGreen)
				} else {
					statusMark(c, iconCross, "连接失败", recRed)
				}
			}
			ui.Icon(c, iconChevronRight).FontSize(13).TextColor(muted)
		})
	})
	if row.Clicked() {
		a.llmOpenProv = p.ID
	}
}

// baseHost is the host of a base URL, for the folded provider line.
func baseHost(base string) string {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return ""
	}
	return u.Host
}

// providerCard edits one saved provider in place; 收起 folds it back.
func (a *app) providerCard(c *ui.Context, p LLMProvider) {
	a.mu.Lock()
	verifying := a.verifyingLLM && a.llmVerifyID == p.ID
	busy := a.verifyingLLM
	var msg string
	var ok bool
	if a.llmVerifyID == p.ID {
		msg, ok = a.llmVerifyMsg, a.llmVerifyOK
	}
	fetching := a.llmFetching[p.ID]
	a.mu.Unlock()
	id := p.ID
	edit := func(fn func(*LLMProvider)) {
		a.settings.update(func(st *Settings) {
			if q := st.provider(id); q != nil {
				fn(q)
			}
		})
	}
	k := func(s string) string { return "llm-" + id + "-" + s }
	// Open in place of its row, in the same group: its own fields divided as
	// the group's rows are, and a tint marking which provider it is.
	ui.Column(c.Key(k("card"))).Background(selBg.Alpha(0.35)).Children(func() {
		ui.Row(c).Padding(10, 14, 4).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "编辑供应商").FontSize(12).FontWeight(600).TextColor(muted).Grow(1)
			if secondaryButton(c, k("fold"), "收起").Clicked() {
				a.llmOpenProv = ""
			}
		})
		ui.Column(c).Gap(1).Dividers(1, line).Children(func() {
			settingsField(c, k("name"), "名称", p.Name, "", func(v string) { edit(func(q *LLMProvider) { q.Name = v }) })
			settingsField(c, k("base"), "Base URL", p.BaseURL, "https://api.example.com/v1", func(v string) { edit(func(q *LLMProvider) { q.BaseURL = v }) })
			settingsField(c, k("key"), "API Key", p.Key, "", func(v string) { edit(func(q *LLMProvider) { q.Key = v }) })
			settingsField(c, k("models"), "模型", joinModels(p.Models), "逗号分隔，验证连接用第一个", func(v string) {
				edit(func(q *LLMProvider) { q.Models = splitModels(v) })
			})
		})
		ui.Column(c).Padding(6, 14, 12).Gap(10).Children(func() {
			a.endpointHint(c, p.BaseURL)
			a.fetchedModels(c, p)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if busyButton(c, k("verify"), "验证连接", "验证中…", verifying, busy).Clicked() && !busy {
					a.verifyLLM(id)
				}
				if busyButton(c, k("fetch"), "获取模型列表", "获取中…", fetching, fetching).Clicked() && !fetching {
					a.fetchModels(id)
				}
				ui.Spacer(c)
				del := ui.ButtonBase(c.Key(k("del"))).Padding(5, 12).Radius(7).Center().Label("删除")
				del.Background(del.AnimateColor("bg", controlFill(del, ui.Transparent, recRed.Alpha(0.12)), hoverFade))
				pressPose(del)
				del.Children(func() { ui.Text(c, "删除").FontSize(13).FontWeight(500).TextColor(recRed) })
				if del.Clicked() {
					a.delProviderID, a.confirmDelProv = id, true
				}
			})
			if msg != "" {
				if ok {
					statusMark(c, iconCheck, msg, okGreen)
				} else {
					warnLine(c, msg).Selectable()
				}
			}
		})
	})
}

// busyButton is a secondaryButton that turns into a spinner and the words
// for the work while working is true, and takes no clicks while off is.
func busyButton(c *ui.Context, key, label, workingLabel string, working, off bool) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Padding(5, 12).Radius(7).Center().Border(1, line).Disabled(off).Label(label)
	fill := hover
	if b.Hovered() {
		fill = press
	}
	b.Background(b.AnimateColor("bg", fill, hoverFade))
	pressPose(b)
	b.Children(func() {
		if working {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c).Size(12, 12)
				ui.Text(c, workingLabel).FontSize(13).TextColor(muted).SingleLine()
			})
			return
		}
		ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(ink).SingleLine()
	})
	return b
}

// fetchedModels shows what 获取模型列表 brought back for p: a filter and a
// chip per model, accented when it is in p's list; a click adds or removes
// it. Long lists are cut to the first matches so the dialog stays usable —
// the filter narrows them down.
func (a *app) fetchedModels(c *ui.Context, p LLMProvider) {
	const shown = 40
	a.mu.Lock()
	ids := a.llmFetched[p.ID]
	errMsg := a.llmFetchErr[p.ID]
	a.mu.Unlock()
	if errMsg != "" {
		ui.Text(c, errMsg).FontSize(12).TextColor(recRed)
	}
	if len(ids) == 0 {
		return
	}
	k := func(s string) string { return "llm-" + p.ID + "-" + s }
	// Folded to one line unless asked for: a fetched list can run to
	// hundreds of models, and it is only wanted while picking.
	open := a.llmShowFetched[p.ID]
	head := ui.ButtonBase(c.Key(k("fetched-head"))).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Width(14).Children(func() { ui.Icon(c, disclosure(open)).FontSize(12).TextColor(muted) })
			ui.Text(c, "从接口选择（"+strconv.Itoa(len(ids))+" 个）").FontSize(12).TextColor(accent)
		})
	})
	if head.Clicked() {
		a.llmShowFetched[p.ID] = !open
	}
	if !open {
		return
	}
	filter := a.llmFilter[p.ID]
	ui.Column(c.Key(k("fetched"))).Gap(8).Children(func() {
		f := filter
		ti := ui.TextInput(c.Key(k("filter")), &f).FontSize(12).TextColor(ink)
		ti.Placeholder("筛选接口返回的 " + strconv.Itoa(len(ids)) + " 个模型")
		if ti.Changed() {
			a.llmFilter[p.ID] = f
		}
		have := map[string]bool{}
		for _, m := range p.modelNames() {
			have[m] = true
		}
		match := filterModels(ids, filter)
		ui.Row(c).Wrap().Gap(6).Children(func() {
			for i, m := range match {
				if i == shown {
					break
				}
				chip := ui.ButtonBase(c.Key(k("m-"+m))).Padding(4, 10).Radius(999).Border(1, line)
				fg := ink
				if have[m] {
					chip.Background(accent).Border(1, accent)
					fg = onAccent
				} else {
					chip.Background(cardBg2)
				}
				chip.Children(func() { ui.Text(c, m).FontSize(12).TextColor(fg) })
				if chip.Clicked() {
					a.toggleModel(p.ID, m)
				}
			}
		})
		if len(match) > shown {
			ui.Text(c, "还有 "+strconv.Itoa(len(match)-shown)+" 个未显示，输入关键字筛选。").FontSize(12).TextColor(muted)
		} else if len(match) == 0 {
			ui.Text(c, "没有匹配的模型。").FontSize(12).TextColor(muted)
		}
	})
}

// filterModels keeps the ids containing every space-separated word of q,
// ignoring case.
func filterModels(ids []string, q string) []string {
	words := strings.Fields(strings.ToLower(q))
	var out []string
next:
	for _, id := range ids {
		l := strings.ToLower(id)
		for _, w := range words {
			if !strings.Contains(l, w) {
				continue next
			}
		}
		out = append(out, id)
	}
	return out
}
