package films

import (
	"strconv"

	"kando/apps/kando/live"
	"kando/apps/kando/locales"
	"kando/apps/kando/theme"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	nsdesktop "github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

// workCard is a work in the grid, its films the page shows folded into one card (fold), as Plex and the Apple TV app show a show: the poster of the film it goes on with (Continue) over two edges stacked behind it, how many of its films are not watched in its corner, or a check once every one is, and its name and how many films it holds under it. A click opens the work's page (App.ShowWork), a ⌘-click or a ⇧-click selects its films or leaves them out (selectWork), and ⋯ and a right click offer its menu (workMenu).
type workCard struct {
	a    App
	card func() card
	// done is the films the grid shows, in its order, which a ⇧-click selects among; index is where the card is among the cards.
	done  func() []Film
	index func() int
}

// Render implements core.Node.
func (w workCard) Render(cx *core.Scope) core.Node {
	a, c := w.a, w.card
	colors, k := ui.ColorsOf(cx), theme.Of(cx)
	name := func() string { return c().Work.Name }
	films := func() []Film { return c().Films }
	left := live.Func(a, func() int { return unwatched(a, films()) })
	selected := live.Func(a, func() bool { return a.Selection().HasAll(Keys(films())) })
	menu := func() []nsdesktop.MenuItem { return workMenu(cx, a, c(), w.done()) }
	ref := &core.Ref{}
	cx.Effect(func() {
		a.Track()
		if focus := a.PosterFocus(); *focus && w.index() == 0 {
			*focus = false
			ref.Focus()
		}
	})
	// The edges of the cards stacked behind the poster, as a pile of the work's films.
	edge := func(inset core.Points, shade float32) core.Node {
		return core.View{PointerEvents: core.PointerEventsNone,
			Style:     core.Style{Position: core.Absolute, Left: core.Pt(inset), Right: core.Pt(inset), Top: core.Pt(-inset / 2), Height: core.Pt(12), CornerRadius: 10, BorderWidth: 0.5},
			StyleFunc: func() core.Style { return core.Style{Background: k.Thumb(), BorderColor: k.Shade(shade)} }}
	}
	badge := core.Show{
		When: func() bool { return left() > 0 },
		Then: func(*core.Scope) core.Node {
			return core.View{TestID: "work-unwatched", Role: core.RoleImage, LabelFunc: func() string { return locales.FilmsWorkUnwatched(cx, left()) }, PointerEvents: core.PointerEventsNone,
				Style:     core.Style{Position: core.Absolute, Top: core.Pt(9), Right: core.Pt(9), MinWidth: core.Pt(20), Height: core.Pt(20), CornerRadius: 10, Padding: core.Symmetric(0, 6), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
				StyleFunc: func() core.Style { return core.Style{Background: colors.Accent()} },
				Children: []core.Node{core.Text{ContentFunc: func() string { return strconv.Itoa(left()) }, MaxLines: 1,
					Style: core.Style{FontSize: 11, FontWeight: ui.Bold, FontVariant: core.FontTabularNums, Color: onPicture}}}}
		},
		Else: func(*core.Scope) core.Node { return watchedMark(cx) },
	}
	poster := base.Button{
		Ref:       ref,
		TestID:    "work-" + c().Work.Key,
		LabelFunc: func() string { return locales.FilmsWorkLabel(cx, name(), len(films())) },
		Selected:  selected,
		OnPress:   live.On(a, func(e core.PressEvent) { pressWork(cx, a, c(), w.done(), e) }),
		Style: core.Style{Width: core.Pct(100), AspectRatio: 16.0 / 9, CornerRadius: 13, BorderWidth: 0.5, OutlineOffset: 3,
			Shadows: posterShadow, Cursor: core.CursorDefault, Hover: &core.Style{Shadows: posterLifted, Transform: core.Transform{Scale: 1.015}}, FocusVisible: ui.FocusRing(cx, 2)},
		StyleFunc: func() core.Style {
			style := core.Style{Background: k.Thumb(), BorderColor: k.Shade(0.1)}
			if selected() {
				style.OutlineWidth, style.OutlineColor = 3, colors.Accent()
			}
			return style
		},
		Children: []core.Node{clip(13,
			cover(live.Func(a, func() core.ImageSource { return a.Poster(c().Film.VideoPath) })),
			badge,
		)},
	}
	stack := core.View{Style: core.Style{Width: core.Pct(100)}, Children: []core.Node{edge(18, 0.06), edge(9, 0.08), poster}}
	tip := ui.Tooltip{TextFunc: name, Trigger: stack, Style: core.Style{Width: core.Pct(100)}}
	meta := core.Text{ContentFunc: live.Func(a, func() string { return workMeta(locales.For(cx), c()) }), MaxLines: 1, EllipsizeMode: core.EllipsizeTail,
		Style: core.Style{FontSize: 12}, StyleFunc: ui.Colored(colors.Muted)}
	caption := core.View{Style: core.Style{Direction: core.Row, Gap: 6, AlignItems: core.AlignStart}, Children: []core.Node{
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 2}, Children: []core.Node{
			core.Text{ContentFunc: name, MaxLines: 2, EllipsizeMode: core.EllipsizeTail, Style: core.Style{FontSize: 13, FontWeight: ui.Semibold, LineHeight: 17}, StyleFunc: ui.Colored(colors.Text)},
			meta,
		}},
		moreButton{a: a, item: func() Film { return c().Film }, testID: "work-more", menu: menu, label: name},
	}}
	return core.View{
		Style:         core.Style{Width: core.Pct(100), MinWidth: core.Pt(0), Gap: 9},
		OnContextMenu: func(e core.ContextMenuEvent) { nsdesktop.PopupMenu(cx, menu(), e.PagePosition, nil) },
		Children:      []core.Node{tip, caption},
	}
}

// workMeta is the line under a work's name, in the language of m: where its films come from and how many the card holds.
func workMeta(m locales.Messages, c card) string {
	count := m.FilmsWorkEpisodes(len(c.Films))
	if c.Work.List != "" {
		count = m.FilmsWorkVideos(len(c.Films))
	}
	return SourceLabel(m, c.Film) + " · " + count
}

// pressWork is a press of a work's card among done, the films the grid shows: a ⌘-click or a ⇧-click selects its films or leaves them out (selectWork); a plain click, or Space or Return on it, empties the selection and opens the work's page.
func pressWork(cx *core.Scope, a App, c card, done []Film, e core.PressEvent) {
	if !e.Keyboard && selectWork(cx, a.Selection(), Keys(c.Films), Keys(done), e.Modifiers) {
		return
	}
	a.Selection().Clear()
	a.ShowWork(c.Work.Key)
}

// selectWork selects as a click of a work's card with mods held does, keys being its films and order those the page shows: ⌘ selects them all or leaves them all out, ⇧ selects from the film selected last to its last film. It reports whether mods held either.
func selectWork(cx *core.Scope, sel *Selection, keys, order []string, mods core.Modifiers) bool {
	command := mods.Control
	if core.Untracked(cx, cx.Env().AppleKeys) {
		command = mods.Platform
	}
	switch {
	case command:
		sel.ToggleAll(keys)
	case mods.Shift:
		sel.Extend(order, keys[len(keys)-1])
		sel.Add(keys)
	default:
		return false
	}
	return true
}

// workMenu is what can be done with a work's card, from ⋯ or a right click: the selection's menu while more than its films are selected (SelectionMenu); else 打开, going on with it (Continue), what can be done with several films at once, done to the card's films, and 拆开, which shows its films a card each (App.SetWorkApart).
func workMenu(cx *core.Scope, a App, c card, done []Film) []nsdesktop.MenuItem {
	sel := a.Selection()
	keys := Keys(c.Films)
	if sel.HasAll(keys) && sel.Len() > len(keys) {
		return SelectionMenu(cx, a, sel.Of(done))
	}
	live.Do(a, func() { sel.Clear() })()
	w := c.Work
	next, how := Continue(a, w, a.Films())
	items := []nsdesktop.MenuItem{
		{Label: locales.FilmsWorkOpen(cx), OnSelect: live.Do(a, func() { a.ShowWork(w.Key) })},
		{Label: goOnLabel(locales.For(cx), w, next, how), OnSelect: live.Do(a, func() { goOn(a, next, how) })},
		{Separator: true},
	}
	batch := SelectionMenu(cx, a, c.Films)
	// The selection's menu opens with how many films it acts on; a work's says whose they are.
	batch[0].Label = locales.FilmsWorkMenuHeader(cx, w.Name, len(c.Films))
	items = append(items, batch...)
	return append(items, nsdesktop.MenuItem{Label: locales.FilmsWorkSplit(cx), OnSelect: live.Do(a, func() { a.SetWorkApart(w, true) })})
}

// goOn plays f, the film a work goes on with, as how says (Continue): from where it stopped, or from its start when the work starts again.
func goOn(a App, f Film, how int) {
	if how == GoOnAgain {
		a.OpenAt(f, 0)
		return
	}
	a.Open(f)
}

// workJoinMenu is the item of the menu of a film of a work the user split apart that folds the work's films into one card again; none for a film of no such work.
func workJoinMenu(cx *core.Scope, a App, item Film) []nsdesktop.MenuItem {
	w, ok := a.Works().Of(item.Key)
	if !ok || !a.WorkApart(w) {
		return nil
	}
	return []nsdesktop.MenuItem{{Label: locales.FilmsWorkJoin(cx, w.Name), OnSelect: live.Do(a, func() { a.SetWorkApart(w, false) })}}
}

// workIcon is the mark of a work's page's way back to the library.
var workIcon = lucide.ChevronLeft
