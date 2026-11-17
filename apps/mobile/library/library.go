// Package library is the page of a computer's library: its films with their posters, languages, lengths and sizes, and how far each is on the phone, or why none show; pull to refresh reads them again. A film opens its own page.
package library

import (
	"context"

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

type filmsSource struct {
	c   *sync.Client
	gen int
}

// Page is the page of library lib of desktop; open opens a film's page.
func Page(cx *core.Scope, s *session.Session, desktop string, lib sync.Library, open func(f sync.Film), back func()) base.NavPage {
	gen := core.NewSignal(cx, 0)
	pulled := core.NewSignal(cx, false)
	films := core.NewResource(cx, func() (filmsSource, bool) {
		c := s.Client(desktop)
		return filmsSource{c, gen.Get()}, c != nil
	}, func(ctx context.Context, src filmsSource, _ core.ResourceInfo) ([]sync.Film, error) {
		return src.c.Films(ctx, lib.ID)
	})
	cx.Effect(func() {
		if !films.Loading() {
			pulled.Set(false)
		}
	})
	list := func() []sync.Film { v, _ := films.Get(); return v }
	name := func() string {
		if d, ok := s.Desktop(desktop); ok {
			return d.Name
		}
		return ""
	}
	problem := func() session.Problem { return session.ProblemOf(films.Err()) }
	failed := func() bool { return films.Err() != nil && !films.Loading() }
	// shown are the films the computer last listed, none after a failure, nor while it is asked again after one.
	shown := func() []sync.Film {
		if films.Err() != nil {
			return nil
		}
		return list()
	}
	ref := &core.Ref{}
	column := look.Column(cx)
	return ui.NavPage(cx, base.NavPage{Title: lib.Name, TestID: "library-page", Children: []core.Node{
		core.Scroll{TestID: "library-scroll", Ref: ref, Style: core.Style{FlexGrow: 1, FlexShrink: 1},
			OnRefresh:  func() { pulled.Set(true); gen.Update(func(n int) int { return n + 1 }) },
			Refreshing: pulled.Get,
			// The column fills the scroll's height while its content is shorter, so an empty or failed library's message centers in the page as the tabs' do.
			Children: []core.Node{core.View{StyleFunc: func() core.Style { st := column(); st.FlexGrow = 1; return st }, Children: []core.Node{
				core.Show{When: func() bool {
					_, ok := films.Get()
					return films.Loading() && (!ok || films.Err() != nil) && !pulled.Get()
				}, Then: func(cx *core.Scope) core.Node {
					var rows []core.Node
					for i := range 3 {
						rows = append(rows, look.Row{Index: func() int { return i }, Inset: 140,
							Leading: ui.SkeletonBlock{Width: core.Pt(112), Height: core.Pt(63), Index: i},
							Below:   ui.Skeleton{Lines: 2}})
					}
					return look.Group{TestID: "library-loading", Children: rows}
				}},
				core.Show{When: failed, Then: func(cx *core.Scope) core.Node {
					return look.Empty{TestID: "library-problem",
						Icon: func() *core.SVG {
							if problem() == session.Unreachable {
								return lucide.WifiOff()
							}
							return lucide.CircleAlert()
						},
						Title: func() string {
							switch problem() {
							case session.Gone:
								return locales.LibraryGone(cx, name())
							case session.Unpaired, session.Certificate:
								return locales.ComputersUnpaired(cx, name())
							}
							return locales.ComputersUnreachable(cx, name())
						},
						Action: look.Button{Kind: look.Tinted, Text: func() string { return locales.CommonRetry(cx) }, TestID: "library-retry", OnPress: films.Refetch}}
				}},
				core.Show{When: func() bool { _, ok := films.Get(); return ok && films.Err() == nil && len(list()) == 0 }, Then: func(cx *core.Scope) core.Node {
					return look.Empty{TestID: "library-empty", Icon: lucide.Film, Title: func() string { return locales.LibraryEmpty(cx) }}
				}},
				core.Show{When: func() bool { return len(shown()) > 0 }, Then: func(cx *core.Scope) core.Node {
					return look.Group{Children: []core.Node{
						core.Each(shown, func(f sync.Film) string { return f.ID }, func(cx *core.Scope, f func() sync.Film, i func() int) core.Node {
							return filmRow(cx, s, desktop, f, i, open)
						}),
					}}
				}},
			}}},
		},
	}}, ui.NavHeader{OnBack: back, Scroll: ref})
}

// filmRow is a film of the list: its poster, which the computer sends, its title and facts, where its download stands while it is under way, paused or stopped, and a mark once it is on the phone.
func filmRow(cx *core.Scope, s *session.Session, desktop string, f func() sync.Film, i func() int, open func(sync.Film)) core.Node {
	colors := ui.ColorsOf(cx)
	key := func() string { return store.Key(desktop, f().ID) }
	transfer := func() (session.Transfer, bool) { return s.Transfer(key()) }
	done := func() bool { k, ok := s.Film(key()); return ok && k.Done() }
	type posterSource struct {
		c  *sync.Client
		id string
	}
	poster := core.NewResource(cx, func() (posterSource, bool) {
		c := s.Client(desktop)
		return posterSource{c, f().ID}, c != nil && f().Poster != ""
	}, func(ctx context.Context, src posterSource, _ core.ResourceInfo) ([]byte, error) {
		return s.Poster(ctx, src.c, desktop, src.id)
	})
	source := func() core.ImageSource {
		if data, ok := poster.Get(); ok && len(data) > 0 {
			return core.ImageSource{Data: data}
		}
		return core.ImageSource{}
	}
	return look.Row{TestID: "library-film-" + f().ID, Index: i, Inset: 140,
		Leading: look.Poster(cx, source, 112, ""),
		Title:   func() string { return f().Title },
		Detail:  func() string { return look.Facts(f()) },
		Below: core.Show{When: func() bool { _, ok := transfer(); return ok && !done() }, Then: func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Gap: 6, Padding: core.Edges{Top: 4}}, Children: []core.Node{
				core.Text{TestID: "library-film-status-" + f().ID, ContentFunc: func() string { return look.Status(cx, s, key()) }, Style: look.Caption,
					StyleFunc: func() core.Style {
						if t, _ := transfer(); t.State == session.Failed {
							return core.Style{Color: colors.Danger()}
						}
						return core.Style{Color: colors.Accent()}
					}},
				core.Show{When: func() bool { t, ok := transfer(); return ok && t.State != session.Failed }, Then: func(cx *core.Scope) core.Node {
					// TODO(nsgo#145): ui.Progress.Label is a string only (ui/progress.go:12-19), so the bar's spoken name keeps the language it was built in; pass a LabelFunc once Progress has one https://github.com/nsgo-dev/nsgo/issues/145
					return ui.Progress{Label: locales.FilmDownload(cx), Value: func() float32 { t, _ := transfer(); return t.Fraction() * 100 }}
				}},
			}}
		}},
		Trailing: core.Show{When: done, Then: func(cx *core.Scope) core.Node {
			return core.View{LabelFunc: func() string { return locales.FilmDownloaded(cx) }, Role: core.RoleImage, Accessible: true,
				Children: []core.Node{ui.Icon(lucide.CircleCheck, 20, colors.Accent)}}
		}},
		OnPress: func() { open(f()) }}
}
