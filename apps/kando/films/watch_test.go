package films

import (
	"slices"
	"testing"
	"time"

	"kando/apps/kando/locales/testlocale"
	"kando/apps/kando/sidebar"
)

// progress is a Progress kept as plain maps, by video path.
type progress struct {
	length, resume, furthest map[string]time.Duration
}

func (p progress) Duration(path string) time.Duration { return p.length[path] }
func (p progress) ResumeAt(path string) time.Duration { return p.resume[path] }
func (p progress) Furthest(path string) time.Duration { return p.furthest[path] }

// TestWatchStateFollowsWhatThePosterShows checks that a film counts as watched once the check over its poster shows, in progress while its progress bar shows or 继续 picks it up, and not watched otherwise, a film played for less than the 3 seconds that do not count as started, its bar not showing either, included.
func TestWatchStateFollowsWhatThePosterShows(t *testing.T) {
	const length = 100 * time.Second
	cases := []struct {
		name               string
		length, resume, at time.Duration
		want               string
		part               float64
	}{
		{name: "never played", length: length, want: sidebar.WatchUnwatched},
		{name: "played two seconds", length: length, at: 2 * time.Second, want: sidebar.WatchUnwatched},
		{name: "played partway", length: length, at: 40 * time.Second, resume: 40 * time.Second, want: sidebar.WatchInProgress, part: 0.4},
		{name: "played partway, length not known", at: 40 * time.Second, resume: 40 * time.Second, want: sidebar.WatchInProgress},
		{name: "stopped partway, from before Kando kept how far", length: length, resume: 40 * time.Second, want: sidebar.WatchInProgress},
		{name: "played to within 3 seconds of its end", length: length, at: 97 * time.Second, want: sidebar.WatchWatched, part: 0.97},
		{name: "watched, playing again", length: length, at: length, resume: 30 * time.Second, want: sidebar.WatchWatched, part: 1},
		{name: "played to its end, length not known", at: length, want: sidebar.WatchInProgress},
		{name: "a clip shorter than 3 seconds played through", length: 2 * time.Second, at: 2 * time.Second, want: sidebar.WatchWatched, part: 1},
	}
	for _, c := range cases {
		p := progress{length: map[string]time.Duration{"v": c.length}, resume: map[string]time.Duration{"v": c.resume}, furthest: map[string]time.Duration{"v": c.at}}
		item := Film{Key: "v", VideoPath: "v"}
		if got := WatchState(p, item); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
		if seen := watchedOf(p, item); seen.part != c.part || seen.end != (c.want == sidebar.WatchWatched) {
			t.Errorf("%s: the poster shows %+v", c.name, seen)
		}
	}
	items := []Film{{Key: "a", VideoPath: "a"}, {Key: "b", VideoPath: "b"}, {Key: "c", VideoPath: "c"}, {Key: "d", VideoPath: "d"}}
	p := progress{
		length:   map[string]time.Duration{"a": length, "b": length, "c": length, "d": length},
		resume:   map[string]time.Duration{"b": 50 * time.Second},
		furthest: map[string]time.Duration{"b": 50 * time.Second, "c": length, "d": length},
	}
	if got := WatchCounts(items, p); got != (sidebar.WatchCounts{Unwatched: 1, InProgress: 1, Watched: 2}) {
		t.Fatalf("counts %+v", got)
	}
	if got := keysOf(ByWatch(items, p, sidebar.WatchWatched)); got != "c,d" {
		t.Fatalf("watched %s", got)
	}
	if got := keysOf(ByWatch(items, p, sidebar.WatchAll)); got != "a,b,c,d" {
		t.Fatalf("every state %s", got)
	}
	// The grid's films are those of every filter at once.
	items[2].Source, items[3].Source = "https://youtu.be/c", "https://youtu.be/d"
	items[3].Name = "Delta"
	if got := keysOf(Shown(items, p, nil, sidebar.SourceYouTube, sidebar.SpokenAll, sidebar.WatchWatched, "delta")); got != "d" {
		t.Fatalf("shown %s", got)
	}
}

// TestPageFiltersByHowFarFilmsWereWatched checks that the library shows only the films in the watch state the sidebar chose, without the hero, and says so when none is, with 显示全部影片, which shows them all again.
func TestPageFiltersByHowFarFilmsWereWatched(t *testing.T) {
	a := newFakeApp()
	rivers := a.items[0].VideoPath
	a.furthest[rivers] = 125 * time.Second
	app := start(t, a)
	msgs := testlocale.Messages()
	shows := func(keys ...string) {
		t.Helper()
		for _, key := range []string{"done", "algebra"} {
			if want := slices.Contains(keys, key); app.Exists("id:film-"+key) != want {
				t.Fatalf("film %s shown %v, want %v: %q", key, !want, want, app.Texts())
			}
		}
	}
	change(app, a, func() { a.watch = sidebar.WatchWatched })
	shows("done")
	if app.Exists("id:films-hero") {
		t.Fatal("the hero shows while a watch state is chosen")
	}
	change(app, a, func() { a.watch = sidebar.WatchUnwatched })
	shows("algebra")
	// With no card in 处理中 either, the page says that no film is in the state.
	change(app, a, func() {
		a.watch = sidebar.WatchInProgress
		a.items = a.items[:2]
	})
	shows()
	if !app.Exists(whole(msgs.FilmsWatchEmptyInProgress())) {
		t.Fatalf("no word that no film is in progress: %q", app.Texts())
	}
	app.Press("id:films-watch-all")
	wantCalls(t, a, "watch "+sidebar.WatchAll)
	app.Settle()
	shows("done", "algebra")
	// The list shows the films of the state chosen too.
	change(app, a, func() { a.watch, a.view = sidebar.WatchWatched, ViewList })
	if app.Exists(whole("Linear Algebra")) || !app.Exists(whole("Rivers of Europe")) {
		t.Fatalf("the list does not keep to the watch state: %q", app.Texts())
	}
}
