package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/Beeta/voxi/internal/audio"
	"github.com/Beeta/voxi/internal/store"
)

// TestMakeScreenshot rewrites docs/screenshot.png, the picture at the top of
// the README. It is skipped unless VOXI_SHOT=1 is set, so a plain `go test`
// never writes into the working tree:
//
//	VOXI_SHOT=1 go test ./internal/app -run TestMakeScreenshot
//
// The recording it draws is made up here rather than taken from a library, so
// the README never shows anyone's real meeting.
func TestMakeScreenshot(t *testing.T) {
	if os.Getenv("VOXI_SHOT") != "1" {
		t.Skip("set VOXI_SHOT=1 to rewrite docs/screenshot.png")
	}
	dir := t.TempDir()
	lib := openTestLib(t, dir)
	first := lib.Add("周会 · 数据资产", filepath.Join(dir, "a.m4a"), 1110)
	lib.SetTranscript(first.ID, &store.Transcript{Segments: []store.Segment{
		{Start: 0, End: 6, Text: "今天主要过三件事：进度、风险和下周排期。", Speaker: "主持人"},
		{Start: 6, End: 15, Text: "数据同步已经跑通，剩下的是权限校验，预计周三前完成。", Speaker: "甲"},
		{Start: 15, End: 24, Text: "风险是排期偏紧，如果权限那部分卡住，交付要往后推两天。", Speaker: "乙"},
	}})
	lib.SetSummary(first.ID, &store.Summary{
		Overview: "## 会议纪要\n\n- **主题**：周会 · 数据资产\n- **目标**：确认进度、风险与下周排期\n- **结论**：数据同步已跑通，剩余权限校验由甲跟进",
		Highlights: []string{
			"数据同步已跑通，剩余权限校验预计周三前完成",
			"排期偏紧：权限部分若卡住，交付要往后推两天",
		},
		Actions: []string{
			"补齐权限校验（甲，周三前）",
			"把排期风险同步给需求方，确认能否接受延后两天",
		},
	})
	lib.Add("访谈 · 用户反馈", filepath.Join(dir, "b.m4a"), 754)
	lib.Add("复盘 · 上线事故", filepath.Join(dir, "c.m4a"), 1620)
	rec, _ := lib.Get(first.ID)
	set := &settingsStore{
		path: filepath.Join(dir, "settings.json"),
		s:    &Settings{Mic: true, System: true},
	}
	a := newApp(lib, set, audio.NewRecorder())
	a.tab = 1
	a.selected = rec.ID

	// 968x676 is the window's content area (its height less the title bar),
	// at 2x as a Retina window draws it.
	tt := ui.NewTester(a.view, 968, 676)
	tt.SetDark(true)
	tt.SetScale(2)
	tt.Frame()
	tt.Frame()

	out, err := os.Create(filepath.Join("..", "..", "docs", "screenshot.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if err := png.Encode(out, tt.Image()); err != nil {
		t.Fatal(err)
	}
	t.Log("已重写 docs/screenshot.png")
}
