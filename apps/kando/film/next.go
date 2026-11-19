package film

import (
	"time"

	"kando/apps/kando/live"
	"kando/apps/kando/locales"
	"kando/apps/kando/theme"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// Nexter is what plays after the open film, as a course, a disc, a season or a playlist goes on: the card that shows once the film ends, counting down to the next, and the commands that go to the next film and the one before.
type Nexter interface {
	// UpNext is the card, the zero UpNext while it does not show.
	UpNext() UpNext
	// PlayNext plays the next film now, as the card's 立即播放 and Return on it do; CancelNext puts the card away, as its 取消 and Escape do, the film staying at its end.
	PlayNext()
	CancelNext()
	// HasNext reports whether the open film has a film after it, which the transport's 下一部 plays.
	HasNext() bool
}

// UpNext is the card as it shows: the next film's title and poster; Due, when it plays, as the card counts down to it for Countdown; or, while it is still being made, Waiting, how far that has got, in the language the interface shows under the scope it is given, and Due zero.
type UpNext struct {
	Open      bool
	Title     string
	Poster    core.ImageSource
	Due       time.Time
	Countdown time.Duration
	Waiting   func(*core.Scope) string
}

// nextGlass is the card's glass, as the recap's.
var nextGlass = glassSpec{theme.GlassCard, 18}

// nextCard is the card over the bottom right of the picture once the film has ended and another plays after it (App.UpNext), as streaming services show the next episode: 接下来 with the next film's poster and title, how many seconds until it plays or how far it is being made, and 立即播放 and 取消. Return plays it now and Escape puts it away; playing the film again, as Space does at its end, or seeking puts it away too.
func (s *screen) nextCard() core.Node {
	a, cx := s.a, s.cx
	open := live.Memo(cx, a, func() bool { return a.UpNext().Open })
	return core.Show{When: open, Then: func(cx *core.Scope) core.Node {
		up := live.Func(a, a.UpNext)
		cx.OnKeyWith("escape", core.KeyOptions{When: core.KeyAlways}, live.Do(a, a.CancelNext))
		cx.OnKeyWith("enter", core.KeyOptions{When: core.KeyAlways}, live.Do(a, a.PlayNext))
		// left is the whole seconds until the next film plays, rounded up, counted down as each passes.
		left := core.NewSignal(cx, 0)
		tick := NewTimer(cx)
		var count func()
		count = func() {
			due := up().Due
			if due.IsZero() {
				left.Set(0)
				return
			}
			rest := due.Sub(cx.Now())
			left.Set(int((rest + time.Second - 1) / time.Second))
			// The number changes as the time left crosses a whole second.
			if rest > 0 {
				step := rest % time.Second
				if step == 0 {
					step = time.Second
				}
				tick.Set(step, count)
			}
		}
		cx.Effect(func() {
			up()
			count()
		})
		status := func() string {
			u := up()
			if u.Waiting != nil {
				return u.Waiting(cx)
			}
			return locales.FilmNextIn(cx, max(left.Get(), 1))
		}
		muted := core.RGBA(0xFFFFFF, 0.6)
		playLabel := func() string { return locales.FilmNextPlay(cx) }
		cancelLabel := func() string { return locales.FilmNextCancel(cx) }
		poster := core.View{Style: core.Style{Width: core.Pt(128), Height: core.Pt(72), FlexShrink: 0, CornerRadius: 8, Overflow: core.Hidden, Background: core.RGBA(0xFFFFFF, 0.08)},
			Children: []core.Node{core.Image{TestID: "film-next-poster", ResizeMode: core.Cover, SourceFunc: func() core.ImageSource { return up().Poster }, Style: fillStyle}}}
		info := core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 3}, Children: []core.Node{
			core.Text{ContentFunc: func() string { return locales.FilmNextUp(cx) }, Role: core.RoleHeader, Style: core.Style{FontSize: 11, LineHeight: 14, FontWeight: core.FontWeightBold, Color: muted}},
			core.Text{TestID: "film-next-title", ContentFunc: func() string { return up().Title }, MaxLines: 2, Style: core.Style{FontSize: 14, LineHeight: 19, FontWeight: core.FontWeightBold, Color: onGlass}},
			core.Text{TestID: "film-next-status", ContentFunc: status, MaxLines: 2, LiveRegion: core.LiveRegionPolite, Style: core.Style{FontSize: 12, LineHeight: 16, Color: muted, FontVariant: core.FontTabularNums}},
		}}
		buttons := core.View{Style: core.Style{Direction: core.Row, Gap: 8, JustifyContent: core.JustifyEnd}, Children: []core.Node{
			base.Button{TestID: "film-next-cancel", LabelFunc: cancelLabel, OnPress: live.On(a, func(core.PressEvent) { a.CancelNext() }),
				Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOff},
				Style: core.Style{Height: core.Pt(30), Padding: core.Symmetric(0, 14), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: 15, Background: core.RGBA(0xFFFFFF, 0.14),
					Hover: &core.Style{Background: core.RGBA(0xFFFFFF, 0.22)}, FocusVisible: ui.FocusRing(cx, 2)},
				Children: []core.Node{core.Text{ContentFunc: cancelLabel, MaxLines: 1, PointerEvents: core.PointerEventsNone, Style: core.Style{FontSize: 12, LineHeight: 16, Color: onGlass}}}},
			base.Button{TestID: "film-next-play", LabelFunc: playLabel, OnPress: live.On(a, func(core.PressEvent) { a.PlayNext() }),
				Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOff},
				Style: core.Style{Direction: core.Row, Gap: 6, Height: core.Pt(30), Padding: core.Symmetric(0, 14), AlignItems: core.AlignCenter, CornerRadius: 15, Background: core.RGB(0xFFFFFF),
					Hover: &core.Style{Background: core.RGBA(0xFFFFFF, 0.88)}, FocusVisible: ui.FocusRing(cx, 2)},
				Children: []core.Node{
					core.Icon{SVG: iconPlay, Style: core.Style{Width: core.Pt(10), Height: core.Pt(10), Color: core.RGB(0x1C1C20)}},
					core.Text{ContentFunc: playLabel, MaxLines: 1, PointerEvents: core.PointerEventsNone, Style: core.Style{FontSize: 12, LineHeight: 16, FontWeight: core.FontWeightBold, Color: core.RGB(0x1C1C20)}},
				}},
		}}
		card := core.View{TestID: "film-next", Role: core.RoleGroup, LabelFunc: func() string { return locales.FilmNextUp(cx) },
			Style: nextGlass.floating().Merge(core.Style{Width: core.Pct(100), MaxWidth: core.Pt(380), Gap: 12, Padding: core.Symmetric(14, 16)}),
			Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row, Gap: 12, AlignItems: core.AlignCenter}, Children: []core.Node{poster, info}},
				buttons,
			}}
		return core.View{PointerEvents: core.PointerEventsBoxNone,
			Style: core.Style{Position: core.Absolute, Left: core.Pt(24), Bottom: core.Pt(112), Direction: core.Row, JustifyContent: core.JustifyEnd},
			StyleFunc: func() core.Style {
				return core.Style{Right: core.Pt(s.roomRight() + 24), Transition: roomTransition}
			},
			Children: []core.Node{card}}
	}}
}
