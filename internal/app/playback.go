package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/store"
)

// playerBar is the pane's bottom row: the play button, the elapsed and
// total time, and the seek bar between them. It is deliberately short — it
// is a bar under the content, not a panel of its own.
func (a *app) playerBar(c *ui.Context, rec store.Recording) {
	a.ensurePlayer(rec)
	a.mu.Lock()
	p := a.player
	a.mu.Unlock()
	dur := rec.Duration
	playing := false
	if p != nil {
		if d := p.Duration(); d > 0 {
			dur = d
		}
		playing = p.IsPlaying()
		if playing {
			a.mu.Lock()
			a.playPos = p.CurrentTime()
			a.mu.Unlock()
		}
	}
	a.mu.Lock()
	pos := a.playPos
	a.mu.Unlock()
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		btn := ui.ButtonBase(c.Key("play")).Size(32, 32).Radius(999).Center().Background(accent)
		btn.Children(func() {
			ui.Icon(c, playIcon(playing)).FontSize(14).TextColor(onAccent)
		})
		if btn.Clicked() {
			a.togglePlay()
		}
		ui.Text(c, formatSecs(pos)).FontSize(11).TextColor(muted).Width(46)
		sl := ui.Slider(c.Key("seek"), &a.playPos, 0, maxf(dur, 0.001)).Grow(1)
		if sl.Changed() && p != nil {
			a.mu.Lock()
			np := a.playPos
			a.mu.Unlock()
			p.Seek(np)
		}
		ui.Text(c, formatSecs(dur)).FontSize(11).TextColor(muted).Width(46)
	})
}

func (a *app) transcriptView(c *ui.Context, rec store.Recording) {
	a.mu.Lock()
	transcribing := a.transcribing[rec.ID]
	errMsg := a.asrErr[rec.ID]
	a.mu.Unlock()
	ui.Column(c).Gap(12).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Text(c, "转写").FontSize(13).Bold().TextColor(muted).Grow(1)
			if !transcribing {
				label := "开始识别"
				if rec.Transcript != nil {
					label = "重新识别"
				}
				b := ui.ButtonBase(c.Key("asr")).Padding(8, 16).Radius(8).Center().Background(cardBg2).Border(1, accent)
				b.Children(func() { ui.Text(c, label).FontSize(12).Bold().TextColor(accent) })
				if b.Clicked() {
					if rec.Transcript != nil {
						a.confirmRetrans = true
					} else {
						a.transcribe(rec)
					}
				}
			}
		})
		switch {
		case transcribing:
			ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(16).Children(func() {
				ui.Spinner(c)
				ui.Text(c, "识别中…（正在上传音频并等待火山识别，可能需要数十秒）").FontSize(13).TextColor(muted)
			})
		case rec.Transcript == nil:
			ui.Column(c).Padding(16).Radius(10).Background(cardBg).Gap(8).Children(func() {
				ui.Text(c, "尚未识别。点击“开始识别”生成带时间线的文字。").FontSize(13).TextColor(muted)
				if errMsg != "" {
					ui.Text(c, "⚠ "+errMsg).FontSize(12).TextColor(recRed).Selectable()
				}
			})
		default:
			a.segments(c, rec)
		}
	})
	if i := ui.AlertDialog(c.Key("retrans"), &a.confirmRetrans, "重新识别？",
		"将丢弃当前转写（含你的文字编辑和说话人命名），用豆包语音重新生成。", "重新识别", "取消"); i == 0 {
		a.transcribe(rec)
	}
	a.speakerRenameModal(c, rec)
}

func (a *app) segments(c *ui.Context, rec store.Recording) {
	a.mu.Lock()
	cur := a.playPos
	a.mu.Unlock()
	tints := speakerTintIndexes(rec)
	ui.Column(c).Gap(2).Children(func() {
		for i := range rec.Transcript.Segments {
			seg := rec.Transcript.Segments[i]
			active := cur >= seg.Start && cur < seg.End
			a.segmentRow(c, rec, i, seg, active, speakers[tints[seg.Speaker]%speakerTintCount])
		}
	})
}

// speakerTintIndexes numbers a transcript's speakers in the order they first
// speak, so the chips of one recording come out in different colors: the
// first speaker takes the first tint, the next a different one. Order of
// appearance rather than a hash of the name, which would let two speakers in
// one meeting land on the same color.
func speakerTintIndexes(rec store.Recording) map[string]int {
	idx := map[string]int{}
	if rec.Transcript == nil {
		return idx
	}
	for _, seg := range rec.Transcript.Segments {
		if seg.Speaker == "" {
			continue
		}
		if _, seen := idx[seg.Speaker]; !seen {
			idx[seg.Speaker] = len(idx)
		}
	}
	return idx
}
