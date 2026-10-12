package md

import (
	"strconv"

	"github.com/egoist/mygo/ui"
)

// Palette is the colors a document is drawn in. This package does not know
// the app's theme, so the caller passes them.
type Palette struct {
	Text    ui.Color // body text
	Muted   ui.Color // quotes
	Accent  ui.Color // list markers and links
	Surface ui.Color // code and table cell background
	Border  ui.Color // quote bar
	// Size is the body text size in DIPs; headings scale from it.
	Size float32
}

// headingSizes are the sizes of headings of levels 1, 2 and the rest,
// relative to the body text.
var headingSizes = [...]float32{17.0 / 14, 15.0 / 14, 1}

// Render draws blocks into the current parent. The whole document shares one
// selection, so a drag across paragraphs copies them in order.
func Render(c *ui.Context, blocks []Block, p Palette) {
	ui.Column(c).Selectable().Children(func() {
		drawBlocks(c, p, blocks, false, 0)
	})
}

// drawBlocks draws blocks, apart by the room between them, or by none in the
// items of a list (tight). depth counts the lists around them.
func drawBlocks(c *ui.Context, p Palette, blocks []Block, tight bool, depth int) {
	for i, b := range blocks {
		var gap float32
		switch {
		case i == 0 || tight:
		case b.Kind == Heading:
			gap = 14
		case blocks[i-1].Kind == Heading:
			gap = 6
		default:
			gap = 10
		}
		drawBlock(c, p, b, depth).Margin(gap, 0, 0, 0)
	}
}

// drawBlock draws one block.
func drawBlock(c *ui.Context, p Palette, b Block, depth int) ui.Element {
	switch b.Kind {
	case Heading:
		return paragraph(c, p, b.Inlines).FontSize(p.Size*headingSizes[min(max(b.Level, 1), len(headingSizes))-1]).Bold()
	case List:
		return drawList(c, p, b, depth)
	case Quote:
		return ui.Column(c).BorderWidth(0, 0, 0, 3).BorderColor(p.Border).Padding(0, 0, 0, 10).
			Children(func() { drawBlocks(c, p, b.Blocks, false, depth) })
	case Code:
		return ui.ScrollHorizontal(c).Background(p.Surface).Radius(6).Children(func() {
			ui.Text(c, b.Text).Font("monospace").FontSize(p.Size - 1).NoWrap().Padding(8, 10)
		})
	case Rule:
		return ui.Divider(c)
	case Table:
		return drawTable(c, p, b.Rows)
	}
	return paragraph(c, p, b.Inlines)
}

// drawList draws a list, each item under its marker.
func drawList(c *ui.Context, p Palette, b Block, depth int) ui.Element {
	return ui.Column(c).Gap(6).Children(func() {
		for i, item := range b.Items {
			ui.Row(c).AlignItems(ui.Start).Gap(8).Children(func() {
				marker := bullet(depth)
				if b.Ordered {
					marker = strconv.Itoa(i+1) + "."
				}
				ui.Text(c, marker).FontSize(p.Size).TextColor(p.Accent).Width(20).Shrink(0).TextAlign(ui.End)
				ui.Column(c).Grow(1).Children(func() { drawBlocks(c, p, item, true, depth+1) })
			})
		}
	})
}

// drawTable lays the cells out in a grid, the header row in bold.
func drawTable(c *ui.Context, p Palette, rows [][]string) ui.Element {
	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
	}
	if cols == 0 {
		return ui.Column(c)
	}
	tracks := make([]ui.Track, cols)
	for i := range tracks {
		tracks[i] = ui.FitContent()
	}
	return ui.Grid(c).ColumnTracks(tracks...).GapX(20).GapY(6).Children(func() {
		for ri, row := range rows {
			for ci := 0; ci < cols; ci++ {
				cell := ""
				if ci < len(row) {
					cell = row[ci]
				}
				t := ui.Text(c, cell).FontSize(p.Size - 1).TextColor(p.Text)
				if ri == 0 {
					t.Bold()
				}
			}
		}
	})
}

// bullet returns the marker of the items of a list in depth lists, as
// browsers draw them.
func bullet(depth int) string {
	switch depth {
	case 0:
		return "•"
	case 1:
		return "◦"
	}
	return "▪"
}

// paragraph draws the text of a paragraph or heading, which the user may
// select.
func paragraph(c *ui.Context, p Palette, inlines []Inline) ui.Element {
	return ui.RichText(c).TextColor(p.Text).Children(func() { drawInlines(c, p, inlines) })
}

// drawInlines draws the runs of a paragraph. Links open in the browser; the
// parser has already dropped any URL the app should not open.
func drawInlines(c *ui.Context, p Palette, inlines []Inline) {
	for _, in := range inlines {
		switch in.Kind {
		case Text:
			ui.Text(c, in.Text)
		case CodeSpan:
			ui.Text(c, in.Text).Font("monospace").FontSize(p.Size - 1).TextBackground(p.Surface)
		case Emphasis:
			ui.RichText(c).Italic().Children(func() { drawInlines(c, p, in.Children) })
		case Strong:
			ui.RichText(c).Bold().Children(func() { drawInlines(c, p, in.Children) })
		case Link:
			ui.Link(c, "", in.URL).TextColor(p.Accent).Children(func() { drawInlines(c, p, in.Children) })
		}
	}
}
