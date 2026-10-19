package app

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Beeta/voxi/internal/asr"
	"github.com/egoist/mygo/ui"
)

// asrSteps shows a transcription's steps: done ones with how long they took
// and what they found, the one under way with its running time, the failed
// one in red with the reason, and the rest greyed. running is false once it
// has failed, which stops the clock at the failure.
func (a *app) asrSteps(c *ui.Context, p asr.Progress, running bool) {
	cur := p.Current()
	ui.Column(c.Key("asr-steps")).Gap(8).Padding(16).Radius(10).Background(cardBg).Children(func() {
		title, col := "识别中", ink
		if !running {
			title, col = "识别失败", recRed
		}
		total := time.Since(p.Started)
		if !running {
			total = lastEnd(p).Sub(p.Started)
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if running {
				ui.Spinner(c)
			}
			ui.Text(c, title+" · 已用时 "+formatClock(total)).FontSize(13).Bold().TextColor(col).Grow(1)
		})
		for i := range asr.StepNames {
			a.asrStepRow(c, asr.Step(i), p, cur, running)
		}
		if p.TaskID != "" {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "任务 ID").FontSize(11).TextColor(muted)
				ui.Text(c, p.TaskID).FontSize(11).TextColor(muted).Selectable().Grow(1)
				if toolButton(c, "asr-copy-task", "复制", false).Clicked() {
					c.Services().WriteClipboard(p.TaskID)
					a.setStatus("已复制任务 ID")
				}
			})
		}
		ui.Text(c, "每一步都记在应用数据目录的 asr.log 里，出问题时可以把它发给开发者。").FontSize(11).TextColor(muted)
	})
}

// asrStepRow is one step: a mark, its name, its time and what it found.
func (a *app) asrStepRow(c *ui.Context, s asr.Step, p asr.Progress, cur asr.Step, running bool) {
	st := p.Steps[s]
	mark, mc, nc := "○", muted, muted
	var took string
	switch {
	case st.Err != "":
		mark, mc, nc = "✕", recRed, recRed
		took = fmtSecs(st.Ended.Sub(st.Started))
	case !st.Ended.IsZero():
		mark, mc, nc = "✓", okGreen, ink
		took = fmtSecs(st.Ended.Sub(st.Started))
	case !st.Started.IsZero() && running && s == cur:
		mark, mc, nc = "⟳", accent, ink
		took = formatClock(time.Since(st.Started))
	}
	ui.Column(c.Key("asr-step-" + strconv.Itoa(int(s)))).Gap(2).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, mark).FontSize(13).TextColor(mc).Width(16)
			ui.Text(c, asr.StepNames[s]).FontSize(13).TextColor(nc).Width(120)
			ui.Text(c, took).FontSize(12).TextColor(muted).Width(56)
			ui.Text(c, stepDetail(s, st, p)).FontSize(12).TextColor(muted).Grow(1).SingleLine()
		})
		if st.Err != "" {
			ui.Text(c, st.Err).FontSize(12).TextColor(recRed).Selectable().Padding(0, 0, 0, 24)
		}
	})
}

// stepDetail is what a step found or is doing, for the view and the log.
func stepDetail(s asr.Step, st asr.StepState, p asr.Progress) string {
	switch s {
	case asr.StepWait:
		if st.Started.IsZero() {
			return ""
		}
		d := "处理中"
		if p.Queued {
			d = "排队中"
		}
		if !st.Ended.IsZero() && st.Err == "" {
			d = "已完成"
		}
		d += " · 已查询 " + strconv.Itoa(p.Polls) + " 次"
		if p.Size > 0 {
			d += " · 音频已被下载 " + fmtBytes(p.Served) + "/" + fmtBytes(p.Size)
		}
		return d
	}
	return st.Detail
}

// asrSidebarStep is the step a recording's transcription is at, for its row
// in the sidebar: "识别中 · 等待识别结果".
func asrSidebarStep(p asr.Progress) string {
	cur := p.Current()
	if cur < 0 || int(cur) >= len(asr.StepNames) {
		return "识别中"
	}
	return "识别中 · " + asr.StepNames[cur]
}

// lastEnd is when the latest step ended, for the total of a failed run.
func lastEnd(p asr.Progress) time.Time {
	t := p.Started
	for _, s := range p.Steps {
		if s.Ended.After(t) {
			t = s.Ended
		}
	}
	return t
}

func fmtSecs(d time.Duration) string { return fmt.Sprintf("%.1fs", d.Seconds()) }

func fmtBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// asrStatus is the sidebar status of id's transcription under way, or ""
// when there is none.
func (a *app) asrStatus(id string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.transcribing[id] {
		return ""
	}
	return asrSidebarStep(a.asrProgress[id])
}
