package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/zhaorongcun/voxi/internal/tunnel"
)

// settingsModal renders the settings dialog over the window while
// a.showSettings is true: capture sources, the ASR key, and LLM providers.
func (a *app) settingsModal(c *ui.Context) {
	if !a.showSettings {
		return
	}
	s := a.settings.snapshot()
	ui.Modal(c.Key("settings"), &a.showSettings, func() {
		ui.Column(c).Width(560).Gap(14).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "设置").FontSize(18).Bold().TextColor(ink).Grow(1)
				done := ui.ButtonBase(c.Key("set-done")).Padding(6, 14).Radius(8).Center().Background(accent)
				done.Children(func() { ui.Text(c, "完成").FontSize(13).Bold().TextColor(ui.Hex("#ffffff")) })
				if done.Clicked() {
					a.showSettings = false
				}
			})
			ui.Scroll(c.Key("set-scroll")).MaxHeight(560).Children(func() {
				ui.Column(c).Gap(22).Padding(0, 2).Children(func() {
					a.recordSettings(c, s)
					a.asrSettings(c, s)
					a.llmSettings(c, s)
				})
			})
		})
	})
}

// settingsSection is a muted heading over its body.
func (a *app) settingsSection(c *ui.Context, title string, body func()) {
	ui.Column(c).Gap(12).Children(func() {
		ui.Text(c, title).FontSize(13).Bold().TextColor(muted)
		body()
	})
}

// recordSettings toggles the capture sources.
func (a *app) recordSettings(c *ui.Context, s Settings) {
	a.settingsSection(c, "录制", func() {
		a.switchRow(c, "mic", labelMic, "默认输入设备（麦克风、耳机麦）", s.Mic, func(v bool) {
			a.settings.update(func(st *Settings) { st.Mic = v })
		})
		a.switchRow(c, "sys", labelSystem, "扬声器 / 耳机正在播放的声音", s.System, func(v bool) {
			a.settings.update(func(st *Settings) { st.System = v })
		})
		a.recordingsDirRow(c, s)
	})
}

// switchRow is a title/description line with a trailing switch.
func (a *app) switchRow(c *ui.Context, key, title, desc string, on bool, set func(bool)) {
	ui.Row(c.Key(key)).AlignItems(ui.Center).Gap(12).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, title).FontSize(14).TextColor(ink)
			ui.Text(c, desc).FontSize(12).TextColor(muted)
		})
		v := on
		ui.Switch(c.Key(key+"-sw"), &v)
		if v != on {
			set(v)
		}
	})
}

// asrSettings edits the ASR api key and the cloudflared path. Its heading
// links to the 豆包语音 console where the key is issued.
func (a *app) asrSettings(c *ui.Context, s Settings) {
	ui.Column(c).Gap(12).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "语音识别（").FontSize(13).Bold().TextColor(muted)
			ui.Link(c, "豆包语音", "https://console.volcengine.com/speech/new/experience/asr").FontSize(13).Bold()
			ui.Text(c, "）").FontSize(13).Bold().TextColor(muted)
		})
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "API Key").FontSize(12).TextColor(muted)
			key := s.ASRKey
			ti := ui.TextInput(c.Key("asr-key"), &key).FontSize(13).TextColor(ink)
			ti.Placeholder("API Key")
			if ti.Changed() {
				a.settings.update(func(st *Settings) { st.ASRKey = key })
			}
		})
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "cloudflared 路径").FontSize(12).TextColor(muted)
			cf := s.CloudflaredPath
			cti := ui.TextInput(c.Key("asr-cf"), &cf).FontSize(13).TextColor(ink)
			cti.Placeholder(tunnel.DefaultCloudflaredPath)
			if cti.Changed() {
				a.settings.update(func(st *Settings) { st.CloudflaredPath = cf })
			}
			ui.Text(c, "识别用 cloudflared 建临时隧道把音频传给豆包；填其可执行文件路径，默认 "+tunnel.DefaultCloudflaredPath+"。").FontSize(12).TextColor(muted)
		})
	})
}

// llmSettings edits the single OpenAI-compatible endpoint and verifies it.
func (a *app) llmSettings(c *ui.Context, s Settings) {
	a.mu.Lock()
	verifying := a.verifyingLLM
	msg, ok := a.llmVerifyMsg, a.llmVerifyOK
	a.mu.Unlock()
	a.settingsSection(c, "大模型配置", func() {
		a.fieldInput(c, "llm-base", "Base URL", s.LLMBaseURL, func(v string) {
			a.settings.update(func(st *Settings) { st.LLMBaseURL = v })
		})
		a.fieldInput(c, "llm-key", "API Key", s.LLMKey, func(v string) {
			a.settings.update(func(st *Settings) { st.LLMKey = v })
		})
		a.fieldInput(c, "llm-model", "模型", s.LLMModel, func(v string) {
			a.settings.update(func(st *Settings) { st.LLMModel = v })
		})
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			btn := ui.ButtonBase(c.Key("llm-verify")).Padding(8, 14).Radius(8).Center().Background(cardBg2).Border(1, line).Disabled(verifying)
			btn.Children(func() {
				if verifying {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Spinner(c)
						ui.Text(c, "验证中…").FontSize(13).TextColor(muted)
					})
				} else {
					ui.Text(c, "验证连接").FontSize(13).Bold().TextColor(ink)
				}
			})
			if !verifying && btn.Clicked() {
				a.verifyLLM()
			}
			if msg != "" {
				col := recRed
				if ok {
					col = okGreen
				}
				ui.Text(c, msg).FontSize(12).TextColor(col)
			}
		})
	})
}
