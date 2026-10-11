package app

import "github.com/egoist/mygo/ui"

// fieldInput is a labeled single-line input.
func (a *app) fieldInput(c *ui.Context, key, label, val string, set func(string)) {
	ui.Row(c.Key(key)).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(13).TextColor(muted).Width(70)
		v := val
		ti := ui.TextInput(c.Key(key+"-i"), &v).Grow(1).FontSize(13).TextColor(ink)
		if ti.Changed() {
			set(v)
		}
	})
}
