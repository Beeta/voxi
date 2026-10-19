package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/tunnel"
)

// settingsModal renders the settings dialog over the window while
// a.showSettings is true, one page at a time: appearance and capture, the
// ASR key, and the LLM providers.
// windowH is the window's content height, which the dialog sizes its body
// against; 0 means the window has not been measured yet.
func (a *app) settingsModal(c *ui.Context, windowH float32) {
	if !a.showSettings {
		return
	}
	s := a.settings.snapshot()
	body := settingsBodyHeight(windowH)
	ui.Modal(c.Key("settings"), &a.showSettings, func() {
		// 616 is the original 560 grown by 10%, matching the window size in
		// app.go; the scrolling body below shrinks with the window.
		ui.Column(c).Width(616).Gap(16).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "设置").FontSize(18).Bold().TextColor(ink).Grow(1)
				done := ui.ButtonBase(c.Key("set-done")).Padding(6, 16).Radius(8).Center().Background(accent)
				done.Children(func() { ui.Text(c, "完成").FontSize(13).Bold().TextColor(onAccent) })
				if done.Clicked() {
					a.showSettings = false
				}
			})
			// The pages stay above the scrolling body, so switching is always
			// at hand however far a page is scrolled.
			ui.Tabs(c.Key("set-tabs"), &a.settingsTab, settingsPages...).Changed()
			ui.Scroll(c.Key("set-scroll-"+settingsPages[a.settingsTab])).MaxHeight(body).Label("设置内容").Children(func() {
				ui.Column(c).Gap(24).Padding(0, 2).Children(func() {
					switch a.settingsTab {
					case 0:
						a.appearanceSettings(c, s)
						a.recordSettings(c, s)
					case 1:
						a.asrSettings(c, s)
					case 2:
						a.llmSettings(c, s)
					}
				})
			})
			ui.Text(c, a.versionLabel()).FontSize(12).TextColor(muted)
		})
	})
}

// settingsPages are the settings dialog's pages, in tab order. The Q&A
// presets are not among them: they are managed where they are used, on the
// Q&A tab.
var settingsPages = []string{"通用", "语音识别", "大模型"}

// versionLabel is what the dialog's footer shows: the version the app was
// packaged with, or dev when it carries none (a plain `go run`), so a
// screenshot or a bug report always says which build it is.
func (a *app) versionLabel() string {
	if a.version == "" {
		return "版本 dev"
	}
	return "版本 " + a.version
}

// settingsSection is a muted heading over its body.
func (a *app) settingsSection(c *ui.Context, title string, body func()) {
	ui.Column(c).Gap(12).Children(func() {
		ui.Text(c, title).FontSize(13).Bold().TextColor(muted)
		body()
	})
}

// What the settings dialog spends outside its scrolling body.
const (
	// settingsBodyFallback is what the body was fixed at before it followed
	// the window: used until the window is measured.
	settingsBodyFallback = 616
	// settingsBodyChrome is the panel's insets plus the title row, the
	// version footer and the gaps between them, with a little slack;
	// measured at 151 for this dialog.
	settingsBodyChrome = 170
	// settingsBodyFloor keeps the body usable in a window too short to show
	// it all.
	settingsBodyFloor = 240
)

// settingsBodyHeight caps the scrolling body of the settings dialog so the
// whole dialog fits in a window of height windowH. The modal panel is capped
// at the window height less the theme's spacing on both sides, and inside
// that it pads by the spacing and holds a title row with a gap under it;
// what is left is the body's.
func settingsBodyHeight(windowH float32) float32 {
	if windowH <= 0 {
		return settingsBodyFallback
	}
	return float32(maxf(float64(windowH-settingsBodyChrome), settingsBodyFloor))
}

// appearanceSettings picks the light or dark look. It is the first section,
// being the one the whole dialog is drawn in: a change takes effect behind
// the dialog as it is made.
func (a *app) appearanceSettings(c *ui.Context, s Settings) {
	a.settingsSection(c, "外观", func() {
		ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "主题").FontSize(14).TextColor(ink)
				ui.Text(c, "跟随系统，或固定为浅色或深色").FontSize(12).TextColor(muted)
			})
			// Changed is what applies the click and reports it: the control
			// writes the chosen index at the end of the pass, so comparing a
			// copy taken before the call would always find it unchanged.
			idx := appearanceIndex(s.Appearance)
			if ui.Segmented(c.Key("appearance"), &idx, appearanceLabels...).Changed() {
				a.setAppearance(appearanceNames[idx])
			}
		})
	})
}

// setAppearance saves the choice and hands it to the framework, which
// repaints every window with it.
func (a *app) setAppearance(name string) {
	a.settings.update(func(st *Settings) { st.Appearance = name })
	a.setTheme(appearanceSource(name))
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
		// As in appearanceSettings: the switch applies the click at the end
		// of the pass, so only Changed sees the new value.
		v := on
		if ui.Switch(c.Key(key+"-sw"), &v).Changed() {
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
