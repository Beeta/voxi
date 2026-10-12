package app

import "github.com/egoist/mygo/ui"

// loadingRow is a spinner with a label, shown while an AI task runs.
func loadingRow(c *ui.Context, label string) {
	ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(16).Children(func() {
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
