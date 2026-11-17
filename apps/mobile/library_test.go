package main

import (
	"strings"
	"testing"

	"kando/apps/mobile/store"
	"kando/packages/sync"
	"kando/packages/sync/synctest"
)

// Once paired, the Computers tab lists the desktop's libraries, a library its films with their languages, lengths and sizes, and pull to refresh reads them again.
func TestListsTheLibrariesAndTheirFilms(t *testing.T) {
	d := newDesktop(t)
	p := startPhone(t, "")
	p.app.WaitFor("id:welcome", wait)
	p.pair(d)
	desk := "id:computers-desktop-" + store.DesktopID(d.Fingerprint)
	p.app.WaitFor("id:computers-library-lib", wait)
	if got := p.texts(desk); !strings.Contains(got, "Studio") || !strings.Contains(got, "Japanese Class") || !strings.Contains(got, msgs.ComputersFilmCount(1)) {
		t.Fatalf("the desktop's section shows %q", got)
	}
	p.app.Press("id:computers-library-lib")
	p.app.WaitFor("id:library-film-f1", wait)
	if got := p.texts("id:library-film-f1"); !strings.Contains(got, "Morning Talk") || !strings.Contains(got, "日本語 → 简体中文 · 0:10 · 977 KB") {
		t.Fatalf("the film's row shows %q", got)
	}
	d.AddFilm(synctest.Film{Library: "lib", Video: []byte("v"), Film: sync.Film{ID: "f2", Title: "Evening Talk", Spoken: "ja"}})
	p.app.PullToRefresh("id:library-scroll")
	p.app.WaitFor("id:library-film-f2", wait)
}

// A desktop that does not answer says so under its name, and the films downloaded from it still show and open; with films on the phone it opens on the downloads.
func TestOfflineTheDownloadsStillShow(t *testing.T) {
	d := newDesktop(t)
	p := startPhone(t, "")
	p.app.WaitFor("id:welcome", wait)
	p.pair(d)
	p.app.WaitUntil(wait, func() bool { kept, _ := store.Open(p.dir).Desktops(); return len(kept) == 1 })
	d.Close()
	dir := p.dir
	keepTalk(t, dir)
	p = startPhone(t, dir)
	p.app.WaitFor("id:downloads-film-f1", wait)
	p.openTab(computersTab, msgs.ComputersTitle())
	problem := "id:computers-problem-" + store.DesktopID(d.Fingerprint)
	p.app.WaitFor(problem, wait)
	if got, want := p.texts(problem), msgs.ComputersUnreachable("Studio"); !strings.Contains(got, want) {
		t.Fatalf("the desktop's section says %q, want %q", got, want)
	}
	p.openTab(downloadsTab, msgs.DownloadsTitle())
	p.app.Press("id:downloads-film-f1")
	p.app.WaitFor("id:film-play", wait)
}

// A desktop that revoked the phone asks to pair again.
func TestARevokedPhoneIsAskedToPairAgain(t *testing.T) {
	d := newDesktop(t)
	p := startPhone(t, "")
	p.app.WaitFor("id:welcome", wait)
	p.pair(d)
	d.RevokeAll()
	id := store.DesktopID(d.Fingerprint)
	p.app.PullToRefresh("id:computers-scroll")
	p.app.WaitFor("id:computers-pair-again-"+id, wait)
	if got, want := p.texts("id:computers-problem-"+id), msgs.ComputersUnpaired("Studio"); !strings.Contains(got, want) {
		t.Fatalf("the desktop's section says %q, want %q", got, want)
	}
}
