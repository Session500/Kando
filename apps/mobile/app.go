package main

import (
	"kando/apps/mobile/computers"
	"kando/apps/mobile/downloads"
	"kando/apps/mobile/film"
	"kando/apps/mobile/library"
	"kando/apps/mobile/locales"
	"kando/apps/mobile/pair"
	"kando/apps/mobile/player"
	"kando/apps/mobile/session"
	"kando/apps/mobile/store"
	"kando/apps/mobile/theme"
	"kando/apps/mobile/welcome"
	"kando/packages/sync"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

// page is a kind of page of the app's stacks.
type page uint8

const (
	rootPage page = iota
	downloadsPage
	computersPage
	pairPage
	libraryPage
	filmPage
	playerPage
)

// The tabs, in the order the tab bar shows them.
const (
	downloadsTab = iota
	computersTab
)

// route is a page of a stack and what it shows: the computer and library a library's or a film's page is of, the film, and the key of the film the player plays.
type route struct {
	page    page
	desktop string
	library sync.Library
	film    sync.Film
	key     string
}

// App is the app with the session cfg configures: the tabs of the downloads and the computers, each a stack of the pages it opens, under a stack of what covers them, pairing and the player; the welcome screen in the tabs' place before anything is paired or downloaded.
func App(cx *core.Scope, cfg session.Config) core.Node {
	theme.Apply(cx)
	colors := ui.ColorsOf(cx)
	s := session.New(cx, cfg)
	outer := base.NewNav(cx, route{page: rootPage})
	navs := [2]*base.Nav[route]{base.NewNav(cx, route{page: downloadsPage}), base.NewNav(cx, route{page: computersPage})}
	// chosen is the tab the user chose, -1 before they chose one.
	chosen := core.NewSignal(cx, -1)
	openPair := func() { outer.Push(route{page: pairPage}) }
	// Paired, the phone shows the computer it paired with.
	paired := func() { chosen.Set(computersTab); navs[computersTab].PopToRoot(); outer.Pop(1) }
	play := func(key string) { outer.Push(route{page: playerPage, key: key}) }
	tabStack := func(n *base.Nav[route]) func(cx *core.Scope) core.Node {
		return func(cx *core.Scope) core.Node {
			return n.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, r route, _ func() int) base.NavPage {
				back := func() { n.Pop(1) }
				openFilm := func(desktop string, lib sync.Library, f sync.Film) {
					n.Push(route{page: filmPage, desktop: desktop, library: lib, film: f})
				}
				switch r.page {
				case computersPage:
					return computers.Page(cx, s, computers.Actions{Pair: openPair, Library: func(desktop string, lib sync.Library) {
						n.Push(route{page: libraryPage, desktop: desktop, library: lib})
					}})
				case libraryPage:
					return library.Page(cx, s, r.desktop, r.library, func(f sync.Film) { openFilm(r.desktop, r.library, f) }, back)
				case filmPage:
					return film.Page(cx, s, r.desktop, r.library.Name, r.film, play, back)
				}
				return downloads.Page(cx, s, func(f store.Film) { openFilm(f.Desktop, sync.Library{Name: f.Library}, f.Film) })
			})
		}
	}
	shell := func(cx *core.Scope) core.Node {
		// The tab the phone opens on, chosen once as the tabs are built so a film downloaded later does not take the user away from where they are: the downloads when there are any, else the computers.
		first := computersTab
		if len(core.Untracked(cx, s.Films)) > 0 {
			first = downloadsTab
		}
		selected := func() int {
			if c := chosen.Get(); c >= 0 {
				return c
			}
			return first
		}
		change := func(i int) {
			// The tab shown, pressed again, goes back to its first page.
			if i == selected() {
				navs[i].PopToRoot()
				return
			}
			chosen.Set(i)
		}
		// TODO(nsgo): Android's system tab bar takes drawable names only, with no way to bundle the app's own (core/tabbar.go:20-26, TabBar.java:154-170); drawn bar until it can show vector icons
		ios := func(name string) string {
			if cx.Env().OS() == core.OSIOS {
				return name
			}
			return ""
		}
		return ui.TabShell{Label: locales.AppName(cx), Selected: selected, OnChange: change, Tabs: []ui.ShellTab{
			{Text: locales.DownloadsTitle(cx), Icon: lucide.CircleArrowDown, SystemIcon: ios("arrow.down.circle"), TestID: "tab-downloads", Content: tabStack(navs[downloadsTab])},
			{Text: locales.ComputersTitle(cx), Icon: lucide.Monitor, SystemIcon: ios("desktopcomputer"), TestID: "tab-computers", Content: tabStack(navs[computersTab])},
		}}
	}
	// Before the store is read the page stays blank: reading it is quick.
	root := core.Show{When: s.Loaded, Then: func(cx *core.Scope) core.Node {
		return core.Show{When: func() bool { return len(s.Desktops()) > 0 || len(s.Films()) > 0 },
			Then: shell,
			Else: func(cx *core.Scope) core.Node { return welcome.Page(cx, openPair) }}
	}}
	return outer.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, r route, _ func() int) base.NavPage {
		switch r.page {
		case pairPage:
			return pair.Page(cx, s, paired, func() { outer.Pop(1) })
		case playerPage:
			return player.Page(cx, s, r.key, func() { outer.Pop(1) })
		}
		return base.NavPage{TestID: "root", HeaderHidden: true, StyleFunc: ui.Filled(colors.Background), Children: []core.Node{root}}
	})
}
