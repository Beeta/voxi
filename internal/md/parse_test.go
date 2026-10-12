package md

import (
	"strings"
	"testing"
)

// plainText is the text of blocks, for tests.
func plainText(blocks []Block) string {
	var b strings.Builder
	var inlines func([]Inline)
	inlines = func(ins []Inline) {
		for _, in := range ins {
			b.WriteString(in.Text)
			inlines(in.Children)
		}
	}
	var walk func([]Block)
	walk = func(bs []Block) {
		for _, bl := range bs {
			inlines(bl.Inlines)
			b.WriteString(bl.Text)
			for _, item := range bl.Items {
				walk(item)
			}
			walk(bl.Blocks)
		}
	}
	walk(blocks)
	return b.String()
}

// TestSoftBreakBetweenCJKKeepsNoSpace is one of the two local changes to the
// parser this package copies. A newline inside a paragraph reads as a space,
// but not between CJK characters, where none was written: without the fix
// "第一点\n第二点" renders as "第一点 第二点".
func TestSoftBreakBetweenCJKKeepsNoSpace(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"chinese", "第一点\n第二点", "第一点第二点"},
		{"chinese punctuation", "他说：\n「好的」", "他说：「好的」"},
		{"japanese", "これは\nテスト", "これはテスト"},
		{"english keeps its space", "first\nsecond", "first second"},
		{"chinese then english", "总结\nsummary", "总结 summary"},
		{"english then chinese", "summary\n总结", "summary 总结"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := plainText(Parse(tc.src)); got != tc.want {
				t.Errorf("Parse(%q) reads %q, want %q", tc.src, got, tc.want)
			}
		})
	}
}

// TestTableIsRecognised is the other local change: a pipe table would
// otherwise be read as a paragraph, running its rows together into prose.
func TestTableIsRecognised(t *testing.T) {
	src := "| 议题 | 结论 |\n| --- | --- |\n| 排期 | 下周 |\n| 预算 | 待定 |"
	blocks := Parse(src)
	if len(blocks) != 1 {
		t.Fatalf("Parse returned %d blocks, want 1", len(blocks))
	}
	if blocks[0].Kind != Table {
		t.Fatalf("the block is kind %v, want Table", blocks[0].Kind)
	}
	rows := blocks[0].Rows
	if len(rows) != 3 {
		t.Fatalf("the table has %d rows, want 3 (header plus two)", len(rows))
	}
	if rows[0][0] != "议题" || rows[0][1] != "结论" {
		t.Errorf("the header is %q, want [议题 结论]", rows[0])
	}
	if rows[2][0] != "预算" || rows[2][1] != "待定" {
		t.Errorf("the last row is %q, want [预算 待定]", rows[2])
	}
	// The delimiter row is not content.
	for _, r := range rows {
		for _, cell := range r {
			if strings.Contains(cell, "---") {
				t.Errorf("the delimiter row leaked into the table: %q", rows)
			}
		}
	}
}

// TestPipesWithoutDelimiterStayAParagraph: only a delimiter row makes a
// table, so prose that happens to use pipes is left alone.
func TestPipesWithoutDelimiterStayAParagraph(t *testing.T) {
	for _, src := range []string{
		"a | b\nc | d",
		"| a | b |\n| c | d |",
	} {
		blocks := Parse(src)
		if len(blocks) != 1 || blocks[0].Kind != Paragraph {
			t.Errorf("Parse(%q) gave %d blocks with first kind %v, want one paragraph",
				src, len(blocks), blocks[0].Kind)
		}
	}
}

func TestBlockKinds(t *testing.T) {
	cases := []struct {
		name string
		src  string
		kind BlockKind
	}{
		{"heading", "## 标题", Heading},
		{"bullet list", "- 一\n- 二", List},
		{"ordered list", "1. 一\n2. 二", List},
		{"quote", "> 引用", Quote},
		{"fenced code", "```\ncode\n```", Code},
		{"rule", "---", Rule},
		{"paragraph", "普通文字", Paragraph},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocks := Parse(tc.src)
			if len(blocks) == 0 {
				t.Fatalf("Parse(%q) returned no blocks", tc.src)
			}
			if blocks[0].Kind != tc.kind {
				t.Errorf("Parse(%q) gave kind %v, want %v", tc.src, blocks[0].Kind, tc.kind)
			}
		})
	}
}

func TestHeadingLevel(t *testing.T) {
	blocks := Parse("### 三级")
	if len(blocks) != 1 || blocks[0].Level != 3 {
		t.Fatalf("Parse gave %+v, want one level-3 heading", blocks)
	}
	if got := plainText(blocks); got != "三级" {
		t.Errorf("the heading reads %q, want %q", got, "三级")
	}
}

func TestNestedList(t *testing.T) {
	blocks := Parse("- 一\n  - 一之一\n- 二")
	if len(blocks) != 1 || blocks[0].Kind != List {
		t.Fatalf("Parse gave %d blocks, want one list", len(blocks))
	}
	items := blocks[0].Items
	if len(items) != 2 {
		t.Fatalf("the list has %d items, want 2", len(items))
	}
	if len(items[0]) != 2 || items[0][1].Kind != List {
		t.Errorf("the first item does not hold a nested list: %+v", items[0])
	}
}

func TestInlines(t *testing.T) {
	blocks := Parse("**粗** *斜* `code` [链接](https://example.com)")
	if len(blocks) != 1 {
		t.Fatalf("Parse gave %d blocks, want 1", len(blocks))
	}
	kinds := map[InlineKind]bool{}
	var walk func([]Inline)
	walk = func(ins []Inline) {
		for _, in := range ins {
			kinds[in.Kind] = true
			walk(in.Children)
		}
	}
	walk(blocks[0].Inlines)
	for _, want := range []InlineKind{Strong, Emphasis, CodeSpan, Link} {
		if !kinds[want] {
			t.Errorf("the paragraph has no inline of kind %v: %+v", want, blocks[0].Inlines)
		}
	}
}

// TestUnsafeLinksBecomeText: a URL the app must not open renders as its own
// text rather than as a link. The app has no router, so a scheme-less path
// handed to a link would go somewhere unpredictable.
func TestUnsafeLinksBecomeText(t *testing.T) {
	blocks := Parse("[点我](file:///etc/passwd)")
	if len(blocks) != 1 {
		t.Fatalf("Parse gave %d blocks, want 1", len(blocks))
	}
	for _, in := range blocks[0].Inlines {
		if in.Kind == Link {
			t.Fatalf("an unsafe URL survived as a link: %+v", in)
		}
	}
	if got := plainText(blocks); got != "点我" {
		t.Errorf("the paragraph reads %q, want the link text alone", got)
	}
}

func TestSafeURL(t *testing.T) {
	safe := []string{"https://example.com", "http://example.com", "mailto:a@b.com", "HTTPS://EXAMPLE.COM"}
	for _, u := range safe {
		if !SafeURL(u) {
			t.Errorf("SafeURL(%q) = false, want true", u)
		}
	}
	unsafe := []string{"javascript:alert(1)", "file:///etc/passwd", "/relative", "ftp://a.com", "", "example.com"}
	for _, u := range unsafe {
		if SafeURL(u) {
			t.Errorf("SafeURL(%q) = true, want false", u)
		}
	}
}
