package main

import "github.com/egoist/mygo/ui"

// loadingRow is a spinner with a label, shown while an AI task runs.
func loadingRow(c *ui.Context, label string) {
	ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(14).Children(func() {
		ui.Spinner(c)
		ui.Text(c, label).FontSize(13).TextColor(muted)
	})
}

// section is a titled block: a muted heading over its body.
func section(c *ui.Context, title string, body func()) {
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, title).FontSize(13).Bold().TextColor(muted)
		body()
	})
}

// bulletList renders items as selectable bullet points.
func bulletList(c *ui.Context, items []string) {
	ui.Column(c).Gap(6).Children(func() {
		for i, it := range items {
			it := it
			ui.Row(c.Key(i)).Gap(8).AlignItems(ui.Start).Children(func() {
				ui.Text(c, "•").FontSize(14).TextColor(accent)
				ui.Text(c, it).FontSize(14).TextColor(ink).Grow(1).Selectable()
			})
		}
	})
}
