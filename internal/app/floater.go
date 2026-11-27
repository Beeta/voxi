package app

import (
	"fmt"
	"time"

	"github.com/Beeta/voxi/internal/audio"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The floater is a small circle that floats above every app while a
// recording is under way, so a recording left running after the meeting is
// hard to miss: an alarm clock emoji inside a ring whose color warms with
// the length of the recording, hopping every ten minutes. It never says it
// is recording: no red, no text, no timer until the pointer is over it, and
// an alarm clock reads as a timer or a reminder. Only its owner knows what
// it means.
//
// It costs next to nothing: it exists only while recording, its own loop
// wakes once a second and redraws only when what it shows changes (a
// color step every minute, the clock each second only while hovered), the
// bounce animates for under a second every ten minutes, and it is
// excluded from screen sharing and screenshots.

// The floater's sizes, in DIPs. floatBox is the closed circle and the
// pill's height; the window is floatInset larger all round, transparent,
// so the bounce has room to grow. floatPillW is the open window's width.
const (
	floatBox   = 32
	floatInset = 4
	floatWin   = floatBox + 2*floatInset
	floatPillW = 196
	floatRing  = 2.5 // the tinted ring round the circle
	// floatBounceScale is how far the first hop of a bounce grows the
	// circle: 32 × 1.12 is under 36, inside floatWin.
	floatBounceScale = 1.12
)

// floatBounceEvery is how often the closed floater bounces to be seen.
const floatBounceEvery = 10 * time.Minute

// quietRemind is how long a recording may hear nothing before the floater
// pulses and a notification asks whether it is still needed.
const quietRemind = 3 * time.Minute

// The colors the dot runs through as a recording gets longer: a neutral
// gray for the first half hour, warming to amber by the hour and to a deep
// orange by two. Never the record red, which would say what it is.
var (
	floatGray   = ui.Hex("#8e8e93")
	floatAmber  = ui.Hex("#ff9f0a")
	floatOrange = ui.Hex("#ff6a1a")
)

// floatTint is the dot's color after a recording of elapsed.
func floatTint(elapsed time.Duration) ui.Color {
	m := float32(elapsed.Minutes())
	switch {
	case m <= 30:
		return floatGray
	case m <= 60:
		return floatGray.Mix(floatAmber, (m-30)/30)
	case m <= 120:
		return floatAmber.Mix(floatOrange, (m-60)/60)
	default:
		return floatOrange
	}
}

// floatFrame is what the floater shows, as a value: the loop redraws when
// it changes and not otherwise. The tint changes by the minute, the clock
// shows (and so counts) only while the pill is open, and the pulse
// alternates each second only once the recording has gone quiet.
type floatFrame struct {
	minute int
	bounce int // how many floatBounceEvery have been recorded
	paused bool
	pulse  int // 0 steady; 1 or 2 alternate while quiet
	second int // the clock, while the pill is open; -1 otherwise
}

func makeFloatFrame(st audio.State, elapsed, quiet time.Duration, open bool, now time.Time) floatFrame {
	f := floatFrame{
		minute: int(elapsed.Minutes()),
		bounce: int(elapsed / floatBounceEvery),
		paused: st == audio.StatePaused,
		second: -1,
	}
	if quiet >= quietRemind {
		f.pulse = 1 + int(now.Unix()%2)
	}
	if open {
		f.second = int(elapsed.Seconds())
	}
	return f
}

// floaterOptions is the floater's window: a panel, so it floats over every
// app and Space, full-screen ones too, stays out of the Dock and the app
// switcher, and takes a click without bringing the app forward. It opens at
// the top right of the main display the first time, and where it was left
// after that.
func floaterOptions(content mygo.Content, work mygo.Rectangle) mygo.WindowOptions {
	return mygo.WindowOptions{
		Title:         "Voxi",
		Panel:         true,
		Hidden:        true,
		Frameless:     true,
		Transparent:   true,
		DisableShadow: true,
		DisableResize: true,
		Width:         floatWin,
		Height:        floatWin,
		X:             work.X + work.Width - floatWin - 24,
		Y:             work.Y + 96,
		StateKey:      "floater",
		Content:       content,
	}
}

// floatLoop shows the floater while a recording is under way, redraws it
// when what it shows changes, and sends the quiet reminder. It runs for
// the life of the app, waking once a second; idle, that is a lock and a
// comparison.
func (a *app) floatLoop() {
	var (
		shown    bool
		last     floatFrame
		reminded bool
	)
	for now := range time.Tick(time.Second) {
		st, elapsed, _, _ := a.rec.Snapshot()
		active := st == audio.StateRecording || st == audio.StatePaused
		if active != shown {
			shown = active
			reminded = false
			last = floatFrame{minute: -1}
			if !active {
				mygo.RunOnMain(func() { a.syncFloater(false) })
			}
		}
		if !active {
			continue
		}
		quiet := a.rec.QuietFor()
		switch {
		case quiet >= quietRemind && !reminded:
			reminded = true
			go remindQuiet(quiet)
		case quiet < quietRemind:
			reminded = false
		}
		f := makeFloatFrame(st, elapsed, quiet, a.floatOpen.Load(), now)
		redraw := f != last
		last = f
		// Every second while recording, so a floater closed with Escape
		// comes back; with nothing to redraw that is one hop to the main
		// thread and a nil check.
		mygo.RunOnMain(func() {
			if made := a.syncFloater(true); a.floatWin != nil && (made || redraw) {
				a.floatWin.Update(func() {})
			}
		})
	}
}

// syncFloater creates the floater as a recording starts and destroys it as
// the recording ends, so no window lingers between recordings. It reports
// whether it made one. Main thread.
func (a *app) syncFloater(active bool) bool {
	if !active || a.closing {
		if a.floatWin != nil {
			a.floatWin.Destroy()
			a.floatWin = nil
		}
		a.floatOpen.Store(false)
		return false
	}
	if a.floatWin != nil {
		return false
	}
	w := mygo.NewWindow(floaterOptions(ui.View(a.floatView), mygo.Screen.PrimaryDisplay().WorkArea))
	// The state remembered may be the pill's, from a quit while it was
	// open: it always starts as the dot.
	w.SetSize(floatWin, floatWin)
	// Hidden from screen sharing, recording and screenshots: whoever the
	// screen is shown to never sees it.
	w.SetContentProtection(true)
	// Escape in a panel closes it; the next tick brings it back while the
	// recording goes on.
	w.OnClosed(func() {
		if a.floatWin == w {
			a.floatWin = nil
			a.floatOpen.Store(false)
		}
	})
	a.floatWin = w
	w.ShowInactive()
	return true
}

// setFloatOpen grows the dot into the pill or shrinks it back, keeping the
// dot where the user put it: the pill opens toward the middle of the
// screen, so a dot parked at the right edge opens leftward. The height
// stays the same. Called after the frame that saw the pointer come or go,
// on the main thread.
func (a *app) setFloatOpen(open bool) {
	w := a.floatWin
	if w == nil || a.floatOpen.Load() == open {
		return
	}
	a.floatOpen.Store(open)
	b := w.Bounds()
	dx := floatPillW - floatWin
	if open {
		work := mygo.Screen.DisplayNearestPoint(mygo.Point{X: b.X, Y: b.Y}).WorkArea
		a.floatLeft = b.X+b.Width/2 > work.X+work.Width/2
		x := b.X
		if a.floatLeft {
			x -= dx
		}
		w.SetBounds(mygo.Rectangle{X: x, Y: b.Y, Width: floatPillW, Height: floatWin})
	} else {
		x := b.X
		if a.floatLeft {
			x += dx
		}
		w.SetBounds(mygo.Rectangle{X: x, Y: b.Y, Width: floatWin, Height: floatWin})
	}
	w.Update(func() {})
}

// remindQuiet sends the quiet reminder. Its words are as discreet as the
// dot: they say the room is quiet, not that anything is recording.
func remindQuiet(quiet time.Duration) {
	if !mygo.NotificationsSupported() {
		return
	}
	n := mygo.NewNotification(mygo.NotificationOptions{
		ID:    "voxi-quiet",
		Title: "有听有记",
		Body:  remindText(quiet),
	})
	n.Show()
}

// remindText is the quiet reminder's words.
func remindText(quiet time.Duration) string {
	return fmt.Sprintf("已经 %d 分钟没有声音了，还需要继续吗？", int(quiet.Minutes()))
}

// floatView draws the floater: the alarm clock in its ring, or under the
// pointer the pill with the clock, pause / resume and stop.
func (a *app) floatView(c *ui.Context) {
	t := *c.Theme()
	useAppearance(&t)
	c.SetTheme(&t)
	c.Root().Background(ui.Transparent)

	st, elapsed, _, _ := a.rec.Snapshot()
	if st != audio.StateRecording && st != audio.StatePaused {
		return
	}
	paused := st == audio.StatePaused
	f := makeFloatFrame(st, elapsed, a.rec.QuietFor(), false, time.Now())
	open := a.floatOpen.Load()

	// The ring carries the tint; once quiet it fades every other second.
	ring := floatTint(elapsed)
	if f.pulse == 2 {
		ring = ring.Alpha(0.25)
	}
	// The window is the box plus floatInset all round, room for the bounce
	// to grow into without being cut off at the window's edge.
	frame := ui.Row(c).Fill().Padding(floatInset).AlignItems(ui.Stretch)
	var box ui.Element
	frame.Children(func() { box = ui.Row(c.Key("float")).Grow(1) })
	box.AlignItems(ui.Center).
		Background(cardBg.Alpha(0.94)).Border(floatRing, ring).Radius(floatBox / 2)
	switch {
	case !open:
		box.Center()
	case a.floatLeft:
		box.Padding(0, 4, 0, 5).Gap(6)
	default:
		box.Padding(0, 5, 0, 4).Gap(6)
	}
	// Every floatBounceEvery of recording, the closed floater hops twice.
	// Open, the user is already looking at it, and the pill has no room to
	// grow.
	scale := box.Keyframes("bounce", bounceFrames(f.bounce), ui.KeyframeOptions{Run: uint64(f.bounce)})
	if !open {
		box.Scale(scale, scale)
	}
	box.Children(func() {
		// The clock emoji stays under the pointer as the pill opens: at the
		// pill's left end when it opens rightward, at its right end when it
		// opens leftward, so the click that follows never lands on stop.
		if open && a.floatLeft {
			a.floatButtons(c, paused)
		}
		// The emoji and, open, the time drag the floater around; the
		// buttons do not.
		grip := ui.Row(c).Gap(6).AlignItems(ui.Center).Grow(1).DragWindow()
		if !open {
			grip.Center()
		}
		grip.Children(func() {
			if open && a.floatLeft {
				ui.Text(c, formatClock(elapsed)).FontSize(13).FontWeight(600).TextColor(ink).Grow(1)
			}
			floatAlarm(c, paused)
			if open && !a.floatLeft {
				ui.Text(c, formatClock(elapsed)).FontSize(13).FontWeight(600).TextColor(ink)
			}
		})
		if open && !a.floatLeft {
			a.floatButtons(c, paused)
		}
	})
	if hov := box.Hovered(); hov != open && a.floatHover != nil {
		a.floatHover(hov)
	}
}

// bounceFrames is the scale of the closed floater's n-th bounce: two hops,
// the second smaller, over about a second. The 0th, as a recording starts,
// stands still.
func bounceFrames(n int) []ui.Keyframe {
	if n == 0 {
		return []ui.Keyframe{{Value: 1}, {At: time.Millisecond, Value: 1}}
	}
	ms := time.Millisecond
	return []ui.Keyframe{
		{Value: 1},
		{At: 180 * ms, Value: floatBounceScale, Ease: ui.EaseOut},
		{At: 380 * ms, Value: 0.94, Ease: ui.EaseInOut},
		{At: 560 * ms, Value: 1 + (floatBounceScale-1)/2, Ease: ui.EaseOut},
		{At: 760 * ms, Value: 0.98, Ease: ui.EaseInOut},
		{At: 900 * ms, Value: 1, Ease: ui.EaseOut},
	}
}

// floatButtons are the pill's pause / resume and stop.
func (a *app) floatButtons(c *ui.Context, paused bool) {
	icn := iconPause
	if paused {
		icn = iconPlay
	}
	if floatButton(c, "pause", icn).Clicked() {
		if paused {
			a.rec.Resume()
		} else {
			a.rec.Pause()
		}
	}
	if floatButton(c, "stop", iconStop).Clicked() {
		a.stopRecording()
	}
}

// floatAlarm is the alarm clock emoji at the floater's heart, faded while
// paused.
func floatAlarm(c *ui.Context, paused bool) {
	e := ui.Text(c, "⏰").FontSize(16).Shrink(0)
	if paused {
		e.Opacity(0.35)
	}
}

// floatButton is one of the pill's round icon buttons.
func floatButton(c *ui.Context, key string, svg *ui.SVG) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Size(24, 24).Radius(12).Center().Shrink(0)
	b.Background(controlFill(b, ui.Transparent, cardBg3))
	b.Children(func() { ui.Icon(c, svg).FontSize(12).TextColor(ink) })
	pressPose(b)
	return b
}
