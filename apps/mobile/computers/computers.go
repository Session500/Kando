// Package computers is the Computers tab: under each computer the phone is paired with, its libraries, or why they do not show, which pull to refresh reads again; the way to pair another, and a computer's options, which unpair it.
package computers

import (
	"context"
	"maps"
	"slices"

	"kando/apps/mobile/locales"
	"kando/apps/mobile/look"
	"kando/apps/mobile/session"
	"kando/apps/mobile/store"
	"kando/packages/sync"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

// Actions are where the page leads.
type Actions struct {
	Pair    func()
	Library func(desktop string, lib sync.Library)
}

// Page is the tab's root page.
func Page(cx *core.Scope, s *session.Session, act Actions) base.NavPage {
	title := func() string { return locales.ComputersTitle(cx) }
	// gen counts the pulls to refresh, which the computers' sections read their libraries again on; loading is which of them are still reading.
	gen := core.NewSignal(cx, 0)
	loading := core.NewSignalFunc(cx, map[string]bool{}, nil)
	pulled := core.NewSignal(cx, false)
	cx.Effect(func() {
		if !pulled.Get() {
			return
		}
		busy := loading.Get()
		if !slices.ContainsFunc(s.Desktops(), func(d store.Desktop) bool { return busy[d.ID] }) {
			pulled.Set(false)
		}
	})
	ref := &core.Ref{}
	body := core.Show{When: func() bool { return len(s.Desktops()) > 0 },
		Else: func(cx *core.Scope) core.Node {
			return look.Empty{TestID: "computers-empty", Icon: lucide.Monitor, Title: func() string { return locales.WelcomeTitle(cx) },
				Action: look.Button{Kind: look.Tinted, Text: func() string { return locales.WelcomePair(cx) }, TestID: "computers-empty-pair", OnPress: act.Pair}}
		},
		Then: func(cx *core.Scope) core.Node {
			return core.Scroll{TestID: "computers-scroll", Ref: ref, Style: core.Style{FlexGrow: 1, FlexShrink: 1},
				OnRefresh:  func() { pulled.Set(true); gen.Update(func(n int) int { return n + 1 }) },
				Refreshing: pulled.Get,
				Children: []core.Node{core.View{StyleFunc: look.Column(cx), Children: []core.Node{
					core.Each(s.Desktops, func(d store.Desktop) string { return d.ID }, func(cx *core.Scope, d func() store.Desktop, _ func() int) core.Node {
						return section(cx, s, d, gen.Get, act, func(busy bool) {
							id := d().ID
							loading.Update(func(m map[string]bool) map[string]bool { m = maps.Clone(m); m[id] = busy; return m })
						})
					}),
				}}}}
		}}
	pairLabel := func() string { return locales.ComputersPair(cx) }
	trailing := []core.Node{ui.IconButton{TestID: "computers-pair", Icon: lucide.Plus, LabelFunc: pairLabel, Size: 44, OnPress: act.Pair}}
	symbols := func() []core.HeaderSymbol { return []core.HeaderSymbol{{Name: "plus", Label: pairLabel()}} }
	return look.TabRoot(cx, base.NavPage{TestID: "computers-page", Children: []core.Node{body}}, title, trailing, symbols)
}

// libSource is what a computer's libraries are read for: its client, and the pull that asked.
type libSource struct {
	c   *sync.Client
	gen int
}

// section is a paired computer's: its name with its options, and its libraries as it lists them, or why it does not.
func section(cx *core.Scope, s *session.Session, d func() store.Desktop, gen func() int, act Actions, busy func(bool)) core.Node {
	colors := ui.ColorsOf(cx)
	libs := core.NewResource(cx, func() (libSource, bool) {
		c := s.Client(d().ID)
		return libSource{c, gen()}, c != nil
	}, func(ctx context.Context, src libSource, _ core.ResourceInfo) ([]sync.Library, error) {
		return src.c.Libraries(ctx)
	})
	cx.Effect(func() { busy(libs.Loading()) })
	list := func() []sync.Library { v, _ := libs.Get(); return v }
	problem := func() session.Problem { return session.ProblemOf(libs.Err()) }
	failed := func() bool { return libs.Err() != nil && !libs.Loading() }
	// shown are the libraries listed while the computer's answer is not a failure.
	shown := func() []sync.Library {
		if failed() {
			return nil
		}
		return list()
	}
	unpairable := func() bool { p := problem(); return p == session.Unpaired || p == session.Certificate }
	options := func() {
		id, name := d().ID, d().Name
		cx.ShowAlert(core.AlertConfig{Title: locales.ComputersUnpairTitle(cx, name), Message: locales.ComputersUnpairMessage(cx), Sheet: true, Cancelable: true,
			Actions: []core.AlertAction{
				{Label: locales.CommonCancel(cx), Role: core.AlertCancel},
				{Label: locales.ComputersUnpair(cx), Role: core.AlertDestructive},
			}}, func(i int) {
			if i == 1 {
				s.Unpair(id)
			}
		})
	}
	placeholder := func() core.Node {
		return core.View{Style: core.Style{Padding: core.Symmetric(14, 16), MinHeight: core.Pt(52), JustifyContent: core.JustifyCenter}, Children: []core.Node{ui.Skeleton{Lines: 1}}}
	}
	return core.View{TestID: "computers-desktop-" + d().ID, Style: core.Style{Gap: 8}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, MinHeight: core.Pt(44), Padding: core.Edges{Left: 4}}, Children: []core.Node{
			core.Text{ContentFunc: func() string { return d().Name }, Role: core.RoleHeader, MaxLines: 1,
				Style: look.Heading.Merge(core.Style{FlexGrow: 1, FlexShrink: 1}), StyleFunc: ui.Colored(colors.Text)},
			ui.IconButton{TestID: "computers-options-" + d().ID, Icon: lucide.Ellipsis, Variant: ui.IconButtonGhost, Size: 44, IconSize: 20,
				LabelFunc: func() string { return locales.ComputersOptions(cx, d().Name) }, OnPress: options},
		}},
		look.Group{Children: []core.Node{
			core.Show{When: func() bool { return libs.Loading() && len(list()) == 0 }, Then: func(cx *core.Scope) core.Node {
				return core.View{Children: []core.Node{placeholder(), placeholder()}}
			}},
			core.Show{When: failed, Then: func(cx *core.Scope) core.Node {
				return core.View{Children: []core.Node{
					look.Row{TestID: "computers-problem-" + d().ID, Inset: 56, TitleLines: 3,
						Leading: look.IconTile(cx, func() *core.SVG {
							if problem() == session.Unreachable {
								return lucide.WifiOff()
							}
							return lucide.CircleAlert()
						}, look.ToneDanger),
						Title:  func() string { return desktopProblem(cx, problem(), d().Name) },
						Detail: func() string { return d().Addr.String() }},
					core.Show{When: unpairable,
						Then: func(cx *core.Scope) core.Node {
							return look.Row{TestID: "computers-pair-again-" + d().ID, Title: func() string { return locales.ComputersPairAgain(cx) }, TitleTone: look.ToneAccent,
								Index: func() int { return 1 }, OnPress: act.Pair}
						},
						Else: func(cx *core.Scope) core.Node {
							return look.Row{TestID: "computers-retry-" + d().ID, Title: func() string { return locales.CommonRetry(cx) }, TitleTone: look.ToneAccent,
								Index: func() int { return 1 }, OnPress: libs.Refetch}
						}},
				}}
			}},
			core.Show{When: func() bool { _, ok := libs.Get(); return ok && libs.Err() == nil && len(list()) == 0 }, Then: func(cx *core.Scope) core.Node {
				return look.Row{Title: func() string { return locales.ComputersNoLibraries(cx) }, TitleTone: look.ToneMuted}
			}},
			core.Each(shown, func(l sync.Library) string { return l.ID }, func(cx *core.Scope, l func() sync.Library, i func() int) core.Node {
				return look.Row{TestID: "computers-library-" + l().ID, Index: i, Inset: 56, Chevron: true,
					Leading: look.IconTile(cx, lucide.Library, look.ToneAccent),
					Title:   func() string { return l().Name },
					Value:   func() string { return locales.ComputersFilmCount(cx, l().Films) },
					OnPress: func() { act.Library(d().ID, l()) }}
			}),
		}},
	}}
}

// desktopProblem is why a computer's libraries do not show.
func desktopProblem(cx *core.Scope, p session.Problem, name string) string {
	switch p {
	case session.Unpaired:
		return locales.ComputersUnpaired(cx, name)
	case session.Certificate:
		return locales.ComputersCertificate(cx, name)
	}
	return locales.ComputersUnreachable(cx, name)
}
