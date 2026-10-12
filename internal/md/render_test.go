package md

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// testPalette mirrors the app's dark palette, which this package does not
// know about.
var testPalette = Palette{
	Text:    ui.Hex("#f5f5f7"),
	Muted:   ui.Hex("#9a9a9e"),
	Accent:  ui.Hex("#0a84ff"),
	Surface: ui.Hex("#333335"),
	Border:  ui.Hex("#3a3a3c"),
	Size:    14,
}

// render draws src and returns the tester, so a test can read the texts the
// frame ended up with.
func render(t *testing.T, src string) *ui.Tester {
	t.Helper()
	blocks := Parse(src)
	tt := ui.NewTester(func(c *ui.Context) {
		Render(c, blocks, testPalette)
	}, 600, 400)
	tt.Frame()
	return tt
}

// TestRenderDrawsTheText covers the path a summary takes end to end: Markdown
// in, elements out.
func TestRenderDrawsTheText(t *testing.T) {
	tt := render(t, "## 会议纪要\n\n第一点\n第二点\n\n- 待办一\n- 待办二")
	for _, want := range []string{"会议纪要", "第一点第二点", "待办一", "待办二"} {
		if !tt.HasText(want) {
			t.Errorf("the render has no text %q; it has %q", want, tt.Texts())
		}
	}
}

// TestRenderKeepsCJKRunTogether is the CJK fix seen from outside: the two
// lines of a Chinese paragraph read as one run, with no space wedged between
// them.
func TestRenderKeepsCJKRunTogether(t *testing.T) {
	tt := render(t, "第一点\n第二点")
	if tt.HasText("第一点 第二点") {
		t.Error("the soft break became a space between Chinese characters")
	}
	if !tt.HasText("第一点第二点") {
		t.Errorf("the paragraph reads %q, want the two lines run together", tt.Texts())
	}
}

func TestRenderTable(t *testing.T) {
	tt := render(t, "| 议题 | 结论 |\n| --- | --- |\n| 排期 | 下周 |")
	for _, want := range []string{"议题", "结论", "排期", "下周"} {
		if !tt.HasText(want) {
			t.Errorf("the table cell %q is missing; the render has %q", want, tt.Texts())
		}
	}
}

func TestRenderCodeAndQuote(t *testing.T) {
	tt := render(t, "> 引用一句\n\n```\nfmt.Println(1)\n```")
	for _, want := range []string{"引用一句", "fmt.Println(1)"} {
		if !tt.HasText(want) {
			t.Errorf("the render has no text %q; it has %q", want, tt.Texts())
		}
	}
}

func TestRenderLink(t *testing.T) {
	tt := render(t, "[文档](https://example.com)")
	if !tt.HasText("文档") {
		t.Errorf("the link text is missing; the render has %q", tt.Texts())
	}
}

// TestRenderInlineStyles keeps the styled runs from swallowing their text.
func TestRenderInlineStyles(t *testing.T) {
	tt := render(t, "**粗体** 和 *斜体* 和 `code`")
	for _, want := range []string{"粗体", "斜体", "code"} {
		if !tt.HasText(want) {
			t.Errorf("the render has no text %q; it has %q", want, tt.Texts())
		}
	}
}

// TestRenderEmpty draws nothing without complaint: a reply can be empty.
func TestRenderEmpty(t *testing.T) {
	render(t, "")
}
