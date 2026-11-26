package films

import (
	"kando/apps/kando/live"
	"kando/apps/kando/locales"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// A work's page is the library page showing the films of one work, opened from its card (App.ShowWork): its heading, then its films the page's filters and search leave, a card or a row each, in the order they play, as a show's page lists its episodes in Plex. Escape, once nothing is selected, and ⌘[ go back to the library, as the heading's 资料库 does.

// workHeader is the heading of a work's page: the way back to the library, the work's name, how many films it holds and how many of them were watched, and the button that goes on with it (Continue).
type workHeader struct {
	a     App
	model func() shelfModel
}

// Render implements core.Node.
func (h workHeader) Render(cx *core.Scope) core.Node {
	a := h.a
	colors := ui.ColorsOf(cx)
	work := func() *Work { return h.model().work }
	type goOnTo struct {
		film Film
		how  int
	}
	next := live.Memo(cx, a, func() goOnTo {
		if w := work(); w != nil {
			f, how := Continue(a, w, a.Films())
			return goOnTo{f, how}
		}
		return goOnTo{}
	})
	meta := live.Func(a, func() string {
		w := work()
		if w == nil {
			return ""
		}
		var films []Film
		for _, key := range w.Films {
			if f, ok := Find(a.Films(), key); ok {
				films = append(films, f)
			}
		}
		return locales.FilmsWorkWatched(cx, len(films), len(films)-unwatched(a, films))
	})
	back := ui.Button{Variant: ui.ButtonGhost, Size: ui.SizeSmall, TestID: "work-back", Icon: workIcon, TextFunc: func() string { return locales.FilmsWorkBack(cx) },
		OnPress: live.Do(a, func() { a.ShowWork("") })}
	goOnButton := textButton(func(cx *core.Scope) string {
		g := next()
		if w := work(); w != nil {
			return goOnLabel(locales.For(cx), w, g.film, g.how)
		}
		return ""
	}, ui.Button{TestID: "work-continue", OnPress: live.Do(a, func() { g := next(); goOn(a, g.film, g.how) })})
	return core.View{Style: core.Style{Gap: 10, Margin: core.Edges{Bottom: 4}}, Children: []core.Node{
		core.View{Style: core.Style{AlignSelf: core.AlignStart, Margin: core.Edges{Left: -8}}, Children: []core.Node{back}},
		core.View{Style: core.Style{Direction: core.Row, Gap: 16, AlignItems: core.AlignCenter}, Children: []core.Node{
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 4}, Children: []core.Node{
				heading(1, core.Text{TestID: "work-name", ContentFunc: func() string {
					if w := work(); w != nil {
						return w.Name
					}
					return ""
				}, MaxLines: 2, EllipsizeMode: core.EllipsizeTail, Style: core.Style{FontSize: 24, FontWeight: ui.Bold, LetterSpacing: -0.3}, StyleFunc: ui.Colored(colors.Text)}),
				core.Text{TestID: "work-meta", ContentFunc: meta, MaxLines: 1, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
			}},
			goOnButton,
		}},
	}}
}

// workKeys binds, under cx, the keys that go back from a work's page while open says one shows: ⌘[, and Escape once nothing is selected, as Escape first selects nothing (selectionKeys).
func workKeys(cx *core.Scope, a App, open func() bool) core.Node {
	return core.Show{When: open, Then: func(cx *core.Scope) core.Node {
		cx.OnKeyWith("mod+[", core.KeyOptions{When: core.KeyNotEditing}, live.Do(a, func() { a.ShowWork("") }))
		return core.Show{When: live.Func(a, func() bool { return a.Selection().Len() == 0 }), Then: func(cx *core.Scope) core.Node {
			cx.OnKeyWith("escape", core.KeyOptions{When: core.KeyNotEditing}, live.Do(a, func() { a.ShowWork("") }))
			return core.Group{}
		}}
	}}
}
