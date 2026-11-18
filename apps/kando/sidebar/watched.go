package sidebar

import (
	"strconv"

	"kando/apps/kando/live"
	"kando/apps/kando/locales"
	"kando/apps/kando/theme"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

// How far a film has been watched, which the sidebar filters the library by, as Apple TV's library does: WatchAll keeps every film, as SpokenAll keeps every language; the others are films.WatchState's values.
const (
	WatchAll        = "all"
	WatchUnwatched  = "unwatched"
	WatchInProgress = "inProgress"
	WatchWatched    = "watched"
)

// WatchCounts is how many done films are in each watch state, among those the other filters leave (App.WatchCounts).
type WatchCounts struct {
	Unwatched, InProgress, Watched int
}

// Of is the count of state, one of the states other than WatchAll.
func (c WatchCounts) Of(state string) int {
	switch state {
	case WatchUnwatched:
		return c.Unwatched
	case WatchInProgress:
		return c.InProgress
	case WatchWatched:
		return c.Watched
	}
	return 0
}

// watched is the list of the watch states under the languages, 全部 · 未看 · 在看 · 已看完, each but 全部 with how many films choosing it shows. It shows while the open library holds done films, every row even at 0, so the rows stay put as films are watched and a state with none is still one to choose, showing that there are none. Choosing one shows the library page's films in that state, among those of the source and the language chosen above and the search, as all apply; 全部 shows them however far they were watched. Off the library page none is chosen.
type watched struct{ a App }

// watchRow is one row of the list as it shows: comparable, so the list is built again only when a row's label or count changed.
type watchRow struct {
	state, label string
	count        int
}

// watchStates are the rows' states in order.
var watchStates = [...]string{WatchAll, WatchUnwatched, WatchInProgress, WatchWatched}

// Render implements core.Node.
func (w watched) Render(cx *core.Scope) core.Node {
	a := w.a
	rows := core.NewMemoFunc(cx, live.Func(a, func() [len(watchStates)]watchRow { return watchRowsOf(cx, a.WatchCounts()) }), func(x, y [len(watchStates)]watchRow) bool { return x == y })
	// The done films are those 全部影片 counts.
	shown := live.Func(a, func() bool { return a.SourceCount(SourceAll) > 0 })
	current := live.Func(a, func() string {
		if a.Page() == PageFilms {
			return a.WatchFilter()
		}
		return ""
	})
	colors := ui.ColorsOf(cx)
	return core.Show{When: shown, Then: func(cx *core.Scope) core.Node {
		return core.View{
			// The list takes the room its rows need, under the languages, and scrolls when the sidebar is too short for it.
			Style: core.Style{FlexShrink: 1, MinHeight: core.Pt(0), Padding: core.Symmetric(0, inset)},
			Children: []core.Node{ui.Sidebar{
				TestID: "sidebar-watched",
				SectionsFunc: func() []ui.SidebarSection {
					list := rows.Get()
					items := make([]ui.SidebarItem, len(list))
					for i, r := range list {
						items[i] = ui.SidebarItem{ID: r.state, Text: rowName(cx, r.label, r.count, r.state != WatchAll), TestID: "watch-" + r.state, Content: watchContent(colors, theme.Of(cx).Glyph, r)}
					}
					return []ui.SidebarSection{{Title: locales.SidebarWatched(cx), Items: items}}
				},
				Selected: current,
				OnSelect: live.On(a, func(state string) {
					if state != current() {
						a.SetWatchFilter(state)
					}
				}),
			}},
		}
	}}
}

// watchRowsOf is the list's rows for counts, in order.
func watchRowsOf(cx *core.Scope, counts WatchCounts) [len(watchStates)]watchRow {
	var rows [len(watchStates)]watchRow
	for i, state := range watchStates {
		rows[i] = watchRow{state: state, label: watchLabel(cx, state), count: counts.Of(state)}
	}
	return rows
}

// watchLabel names a watch state as its row does.
func watchLabel(cx *core.Scope, state string) string {
	switch state {
	case WatchUnwatched:
		return locales.SidebarWatchUnwatched(cx)
	case WatchInProgress:
		return locales.SidebarWatchInProgress(cx)
	case WatchWatched:
		return locales.SidebarWatchWatched(cx)
	}
	return locales.SidebarWatchAll(cx)
}

// watchIcon is a row's mark: an eye for 全部, then a circle that fills in as a film is watched, an empty one, a dashed one and one with the check the posters of watched films carry.
func watchIcon(state string) func() *core.SVG {
	switch state {
	case WatchUnwatched:
		return lucide.Circle
	case WatchInProgress:
		return lucide.CircleDashed
	case WatchWatched:
		return lucide.CircleCheck
	}
	return lucide.Eye
}

// watchContent is a row's mark, label and count, as sourceContent draws a source's; 全部 has no count.
func watchContent(colors ui.Colors, glyph func() core.Color, r watchRow) core.Node {
	label := ui.Text(r.label, 13, ui.Regular, colors.Text)
	label.MaxLines, label.Style.FlexGrow, label.Style.FlexShrink = 1, 1, 1
	children := []core.Node{ui.Icon(watchIcon(r.state), 16, glyph), label}
	if r.state != WatchAll {
		count := ui.Tabular(ui.Text(strconv.Itoa(r.count), 12, ui.Regular, colors.Muted))
		count.TestID, count.Style.FlexShrink = "watch-count-"+r.state, 0
		children = append(children, count)
	}
	return core.Group(children)
}
