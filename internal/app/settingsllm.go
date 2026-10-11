package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/llm"
)

// fieldInput is a labeled single-line input.
func (a *app) fieldInput(c *ui.Context, key, label, val string, set func(string)) {
	ui.Row(c.Key(key)).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(13).TextColor(muted).Width(70)
		v := val
		ti := ui.TextInput(c.Key(key+"-i"), &v).Grow(1).FontSize(13).TextColor(ink)
		if ti.Changed() {
			set(v)
		}
	})
}

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
