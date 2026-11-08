package app

import (
	"github.com/Beeta/voxi/internal/tunnel"
	"github.com/egoist/mygo/ui"
)

// settingsModal renders the settings dialog over the window while
// a.showSettings is true, laid out as the system's settings are: the pages
// in a sidebar on the left, the one chosen on the right under its title,
// its settings in rounded groups. The pages are appearance and capture, the
// ASR key, and the LLM providers.
// windowH is the window's content height, which the dialog sizes its body
// against; 0 means the window has not been measured yet.
func (a *app) settingsModal(c *ui.Context, windowH float32) {
	if !a.showSettings {
		return
	}
	s := a.settings.snapshot()
	// One height for every page, so the panel does not jump as the sidebar
	// switches them; a tall window gets no taller a panel than the pages
	// need.
	body := min(settingsBodyHeight(windowH), settingsBodyMax)
	w, _ := c.Size()
	ui.DialogBase(c.Key("settings"), &a.showSettings, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.4))
		panel.Width(min(settingsPanelWidth, w-40)).Height(body+settingsHeaderHeight).Radius(14).
			Background(pageBg).Border(1, line).Shadow(0, 16, 40, 0, ui.RGBA(0, 0, 0, 0.3))
		ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
			a.settingsSidebar(c)
			ui.Column(c).Width(1).Shrink(0).Background(line)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				ui.Row(c).Height(settingsHeaderHeight).Padding(0, 20, 0, 24).AlignItems(ui.Center).Children(func() {
					ui.Text(c, settingsPages[a.settingsTab]).FontSize(17).FontWeight(700).TextColor(ink).Grow(1).SingleLine()
					done := ui.ButtonBase(c.Key("set-done")).Padding(5, 14).Radius(7).Center().Background(accent).Label("完成")
					pressPose(done)
					done.Children(func() { ui.Text(c, "完成").FontSize(13).FontWeight(600).TextColor(onAccent) })
					if done.Clicked() {
						a.showSettings = false
					}
				})
				ui.Scroll(c.Key("set-scroll-" + settingsPages[a.settingsTab])).Height(body).Label("设置内容").Children(func() {
					ui.Column(c).Gap(22).Padding(4, 24, 24).Children(func() {
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
			})
		})
	})
}

// settingsSidebar lists the pages, the one shown marked, with the version
// at its foot: out of the way, and where a screenshot of any page still
// shows it.
func (a *app) settingsSidebar(c *ui.Context) {
	// A veil over the panel rather than a color of its own, as the main
	// window's sidebar is the material and not a panel. Its corners follow
	// the panel's on the left, which it would otherwise square off.
	ui.Column(c).Width(settingsSidebarWidth).Shrink(0).Padding(18, 10, 14).Gap(2).Radius(14, 0, 0, 14).Background(hover).Children(func() {
		ui.Text(c, "设置").FontSize(12).FontWeight(600).TextColor(muted).Padding(0, 8, 6)
		for i := range settingsPages {
			a.settingsNavItem(c, i)
		}
		ui.Spacer(c)
		ui.Text(c, a.versionLabel()).FontSize(11).TextColor(muted).Padding(0, 8)
	})
}

// settingsNavItem opens page i of the settings.
func (a *app) settingsNavItem(c *ui.Context, i int) {
	name := settingsPages[i]
	selected := a.settingsTab == i
	b := ui.ButtonBase(c.Key("set-nav-" + name)).Height(30).PaddingX(8).Gap(8).Radius(7).
		AlignItems(ui.Center).Justify(ui.Start).Label(name)
	bg := ui.Transparent
	switch {
	case selected:
		bg = press
	case b.Hovered():
		bg = hover
	}
	b.Background(b.AnimateColor("bg", bg, hoverFade))
	col := muted
	if selected {
		col = accent
	}
	b.Children(func() {
		ui.Icon(c, settingsIcons[i]).FontSize(15).TextColor(col)
		ui.Text(c, name).FontSize(13).FontWeight(500).TextColor(ink).SingleLine()
	})
	if b.Clicked() {
		a.settingsTab = i
	}
}

// settingsPages are the settings dialog's pages, in tab order. The Q&A
// presets are not among them: they are managed where they are used, on the
// Q&A tab.
var settingsPages = []string{"通用", "语音识别", "大模型"}

// settingsIcons are the pages' icons in the sidebar, in settingsPages' order.
var settingsIcons = []*ui.SVG{iconGear, iconMic, iconSparkle}

// The settings dialog's frame.
const (
	// settingsPanelWidth is the panel's width where the window has room for
	// it; in a narrower one it keeps 20 clear on each side.
	settingsPanelWidth = 760
	// settingsSidebarWidth is the width of the list of pages.
	settingsSidebarWidth = 168
	// settingsHeaderHeight is the strip over a page holding its title and
	// 完成, the whole of the panel outside its scrolling body.
	settingsHeaderHeight = 52
	// settingsBodyMax keeps the panel from growing past what the longest
	// page needs in a tall window.
	settingsBodyMax = 540
)

// versionLabel is what the dialog's footer shows: the version the app was
// packaged with, or dev when it carries none (a plain `go run`), so a
// screenshot or a bug report always says which build it is.
func (a *app) versionLabel() string {
	if a.version == "" {
		return "版本 dev"
	}
	return "版本 " + a.version
}

// settingsGroup is a group of settings as the system's settings draw one:
// its title over a rounded card, the rows divided by hairlines, and a line
// under the card for what the rows leave unsaid. footer may be nil.
func settingsGroup(c *ui.Context, key, title string, rows, footer func()) {
	ui.Column(c.Key(key)).Gap(6).Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(muted).Padding(0, 4)
		}
		ui.Column(c).Gap(1).Radius(10).Background(cardBg).Border(1, line).Dividers(1, line).Children(rows)
		if footer != nil {
			ui.Row(c).Padding(2, 4, 0).Gap(4).AlignItems(ui.Center).Wrap().Children(footer)
		}
	})
}

// settingsRow is one setting in a group: what it is and, under it, what it
// means, with its control at the end of the row.
func settingsRow(c *ui.Context, key, label, desc string, control func()) {
	ui.Row(c.Key(key)).Padding(10, 14).Gap(16).AlignItems(ui.Center).MinHeight(44).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(ink)
			if desc != "" {
				ui.Text(c, desc).FontSize(12).TextColor(muted).LineHeight(1.35)
			}
		})
		control()
	})
}

// What the settings dialog spends outside its scrolling body.
const (
	// settingsBodyFallback is what the body was fixed at before it followed
	// the window: used until the window is measured.
	settingsBodyFallback = 616
	// settingsBodyChrome is what the panel spends outside its scrolling body
	// — the header strip and its border — plus 20 of room above and below
	// it in the window, with a little slack.
	settingsBodyChrome = 100
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

// appearanceSettings picks the light or dark look. It is the first group,
// being the one the whole dialog is drawn in: a change takes effect behind
// the dialog as it is made.
func (a *app) appearanceSettings(c *ui.Context, s Settings) {
	settingsGroup(c, "set-look", "外观", func() {
		settingsRow(c, "theme", "主题", "跟随系统，或固定为浅色或深色", func() {
			// Changed is what applies the click and reports it: the control
			// writes the chosen index at the end of the pass, so comparing a
			// copy taken before the call would always find it unchanged.
			idx := appearanceIndex(s.Appearance)
			if segmented(c, "appearance", &idx, appearanceLabels...) {
				a.setAppearance(appearanceNames[idx])
			}
		})
	}, nil)
}

// setAppearance saves the choice and hands it to the framework, which
// repaints every window with it.
func (a *app) setAppearance(name string) {
	a.settings.update(func(st *Settings) { st.Appearance = name })
	a.setTheme(appearanceSource(name))
}

// recordSettings toggles the capture sources and says where recordings go.
func (a *app) recordSettings(c *ui.Context, s Settings) {
	settingsGroup(c, "set-rec", "录制", func() {
		a.switchRow(c, "mic", labelMic, "默认输入设备（麦克风、耳机麦）", s.Mic, func(v bool) {
			a.settings.update(func(st *Settings) { st.Mic = v })
		})
		a.switchRow(c, "sys", labelSystem, "扬声器 / 耳机正在播放的声音", s.System, func(v bool) {
			a.settings.update(func(st *Settings) { st.System = v })
		})
		a.recordingsDirRow(c, s)
	}, func() {
		ui.Text(c, "改到别的目录时可以选择把已有录音一起搬过去；不搬的话旧录音留在原处，仍可播放和识别。").FontSize(11.5).TextColor(muted)
	})
}

// switchRow is a setting turned on and off by a switch at its end.
func (a *app) switchRow(c *ui.Context, key, title, desc string, on bool, set func(bool)) {
	settingsRow(c, key, title, desc, func() {
		// As in appearanceSettings: the switch applies the click at the end
		// of the pass, so only Changed sees the new value.
		v := on
		if ui.Switch(c.Key(key+"-sw"), &v).Label(title).Changed() {
			set(v)
		}
	})
}

// asrSettings edits the ASR api key and the cloudflared path. The footer
// links to the 豆包语音 console where the key is issued.
func (a *app) asrSettings(c *ui.Context, s Settings) {
	settingsGroup(c, "set-asr", "豆包语音", func() {
		settingsField(c, "asr-key", "API Key", s.ASRKey, "在豆包语音控制台创建", func(v string) {
			a.settings.update(func(st *Settings) { st.ASRKey = v })
		})
	}, func() {
		ui.Text(c, "Key 在").FontSize(11.5).TextColor(muted)
		ui.Link(c, "豆包语音控制台", "https://console.volcengine.com/speech/new/experience/asr").FontSize(11.5)
		ui.Text(c, "申请，录音文件识别用它鉴权。").FontSize(11.5).TextColor(muted)
	})
	settingsGroup(c, "set-tunnel", "隧道", func() {
		settingsField(c, "asr-cf", "cloudflared 路径", s.CloudflaredPath, tunnel.DefaultCloudflaredPath, func(v string) {
			a.settings.update(func(st *Settings) { st.CloudflaredPath = v })
		})
	}, func() {
		ui.Text(c, "识别时用 cloudflared 建临时隧道把音频传给豆包；留空即用默认路径 "+tunnel.DefaultCloudflaredPath+"。").FontSize(11.5).TextColor(muted)
	})
}

// settingsField is a setting typed in: its label on the left, and the input
// filling the rest of the row, saving as the user types.
func settingsField(c *ui.Context, key, label, val, placeholder string, set func(string)) {
	ui.Row(c.Key(key+"-row")).Padding(8, 14).Gap(12).AlignItems(ui.Center).MinHeight(44).Children(func() {
		ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(ink).Width(110).Shrink(0)
		v := val
		ti := ui.TextInput(c.Key(key), &v).Grow(1).MinWidth(0).FontSize(13).TextColor(ink).Placeholder(placeholder).Label(label)
		if ti.Changed() {
			set(v)
		}
	})
}
