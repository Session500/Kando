package main

import (
	"strings"
	"testing"
	"time"

	"kando/apps/mobile/store"
	"kando/packages/sync"
	"kando/packages/sync/synctest"
)

// newDesktopWithShow is newDesktop with River Walk's three episodes, local films its computer folds into one work, the first watched there to its end, whose titles name the harbor and not the show.
func newDesktopWithShow(t *testing.T) *synctest.Desktop {
	t.Helper()
	d := newDesktop(t)
	for i, title := range []string{"Harbor Dawn", "Harbor Dusk", "Lighthouse"} {
		f := sync.Film{ID: "e" + string(rune('1'+i)), Title: title, Source: "local", DurationMS: 60_000, Spoken: "ja", Updated: now().Add(-time.Duration(i+1) * time.Minute),
			Work: "w1", WorkTitle: "River Walk", WorkIndex: i + 1}
		if i == 0 {
			f.Watched, f.PlayedAt = true, now().Add(-time.Hour)
		}
		d.AddFilm(synctest.Film{Library: "lib", Video: talkVideo, Cues: talkCues, Film: f})
	}
	return d
}

// workRow selects the row of work id of desktop in the Library.
func workRow(desktop, id string) string { return "id:films-work-" + desktop + "-" + id }

// The Library folds a work's episodes into one row, which says where they come from, how many there are and how many were not watched, and opens the work's page: its episodes in the order they play, how many were watched, and the episode it goes on with, the one after the episode watched on the computer.
func TestTheLibraryFoldsAWorksEpisodes(t *testing.T) {
	d := newDesktopWithShow(t)
	dir := t.TempDir()
	p := startPhone(t, dir, pairedWith(t, d, dir))
	id := store.DesktopID(d.Fingerprint)
	row := workRow(id, "w1")
	p.app.WaitFor(row, wait)
	for _, ep := range []string{"e1", "e2", "e3"} {
		if p.app.Exists(filmRow(id, ep)) {
			t.Fatalf("episode %s has a row of its own", ep)
		}
	}
	if got, want := p.texts("id:films-work-facts-"+id+"-w1"), msgs.FilmsSourceLocal()+" · "+msgs.FilmsWorkEpisodes(3); got != want {
		t.Fatalf("the work's row says %q, want %q", got, want)
	}
	if got := p.texts("id:films-work-unwatched-" + id + "-w1"); got != "2" {
		t.Fatalf("the row counts %q not watched", got)
	}
	if got := p.texts("id:films-count"); got != msgs.FilmsCount(4) {
		t.Fatalf("the list counts %q, still by film", got)
	}
	p.app.Press(row)
	p.app.WaitFor("id:work-page", wait)
	if got := p.texts("id:work-title"); got != "River Walk" {
		t.Fatalf("the page is titled %q", got)
	}
	if got, want := p.texts("id:work-watched"), msgs.FilmsWorkWatched(3, 1); got != want {
		t.Fatalf("the page says %q, want %q", got, want)
	}
	if got, want := p.texts("id:work-go-on"), msgs.FilmsWorkContinue(2); got != want {
		t.Fatalf("the page offers %q, want %q", got, want)
	}
	if got := p.texts("id:work-films"); strings.Index(got, "Harbor Dawn") > strings.Index(got, "Harbor Dusk") || strings.Index(got, "Harbor Dusk") > strings.Index(got, "Lighthouse") {
		t.Fatalf("the page lists %q, not in the work's order", got)
	}
	p.app.Press("id:work-go-on")
	p.app.WaitFor("id:film-download", wait)
	if got := p.texts("id:film-title"); got != "Harbor Dusk" {
		t.Fatalf("going on opened %q", got)
	}
}

// A search for the work's name finds it as one row; a search for some of its episodes' titles finds them as rows of their own, so the list shows which it found.
func TestASearchFoldsTheWorkItFoundByName(t *testing.T) {
	d := newDesktopWithShow(t)
	dir := t.TempDir()
	p := startPhone(t, dir, pairedWith(t, d, dir))
	id := store.DesktopID(d.Fingerprint)
	p.app.WaitFor(workRow(id, "w1"), wait)
	p.app.Type("id:films-search", "river")
	p.app.WaitUntil(wait, func() bool { return p.texts("id:films-count") == msgs.FilmsCount(3) })
	if !p.app.Exists(workRow(id, "w1")) || p.app.Exists(talkRow(id)) {
		t.Fatal("a search for the work's name does not find it as one row")
	}
	p.app.Clear("id:films-search")
	p.app.Type("id:films-search", "harbor")
	p.app.WaitFor(filmRow(id, "e2"), wait)
	if !p.app.Exists(filmRow(id, "e1")) || p.app.Exists(workRow(id, "w1")) || p.app.Exists(filmRow(id, "e3")) {
		t.Fatal("a search for two episodes' titles does not find them as rows of their own")
	}
}

// The phone keeps each film's work as its computer last listed it, so away from the computer the films it keeps still fold; and an open Library folds anew as soon as the computer lists a film otherwise (sync.EventFilms), as once its user showed the work's films apart there.
func TestThePhoneKeepsTheWorkItsComputerNamed(t *testing.T) {
	d := newDesktopWithShow(t)
	dir := t.TempDir()
	secrets := pairedWith(t, d, dir)
	id := store.DesktopID(d.Fingerprint)
	st := store.Open(dir)
	for _, ep := range []string{"e1", "e2"} {
		if err := st.SaveFilm(store.Film{Desktop: id, Library: "Japanese Class", Video: "video.mp4", Added: now(), Film: sync.Film{ID: ep, Title: "kept " + ep, DurationMS: 60_000}}); err != nil {
			t.Fatal(err)
		}
	}
	p := startPhone(t, dir, secrets)
	p.app.WaitFor(workRow(id, "w1"), wait)
	p.app.WaitUntil(wait, func() bool {
		films, _ := st.Films()
		n := 0
		for _, f := range films {
			if f.Film.Work == "w1" && f.Film.WorkTitle == "River Walk" && (f.Film.ID != "e1" || f.Film.Watched) {
				n++
			}
		}
		return n == 2
	})

	// The computer shows the work's films apart: the Library lists them a row each.
	for _, ep := range []string{"e1", "e2", "e3"} {
		d.ListFilm(ep, func(f *sync.Film) { f.Work, f.WorkTitle, f.WorkIndex = "", "", 0 })
	}
	p.app.WaitFor(filmRow(id, "e3"), wait)
	if p.app.Exists(workRow(id, "w1")) {
		t.Fatal("the work's row stays once its computer shows its films apart")
	}
}

// A film its computer lists otherwise between the phone's reads of its films and of its queue, which the phone follows the events from, is told by no event the phone hears: the phone reads the films again once it follows them, so the Library still folds as the computer does.
func TestTheLibraryFoldsAnewAFilmListedOtherwiseBeforeThePhoneFollowsItsComputer(t *testing.T) {
	d := newDesktopWithShow(t)
	release := d.HoldJobs()
	dir := t.TempDir()
	p := startPhone(t, dir, pairedWith(t, d, dir))
	id := store.DesktopID(d.Fingerprint)
	p.app.WaitFor(workRow(id, "w1"), wait)
	for _, ep := range []string{"e1", "e2", "e3"} {
		d.ListFilm(ep, func(f *sync.Film) { f.Work, f.WorkTitle, f.WorkIndex = "", "", 0 })
	}
	release()
	p.app.WaitFor(filmRow(id, "e3"), wait)
	if p.app.Exists(workRow(id, "w1")) {
		t.Fatal("the work's row stays once its computer showed its films apart")
	}
}
