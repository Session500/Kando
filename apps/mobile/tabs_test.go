package main

import (
	"testing"

	"kando/apps/mobile/store"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// With only a computer paired the phone opens on the computers; each tab keeps the pages it opened while the other shows, a film downloaded meanwhile does not take the user to the downloads, and the tab shown, pressed again, goes back to its first page.
func TestTheTabsKeepTheirPagesAndGoBackWhenPressedAgain(t *testing.T) {
	d := newDesktop(t)
	dir := t.TempDir()
	p := startPhone(t, dir, pairedWith(t, d, dir))
	p.app.WaitFor("id:computers-library-lib", wait)
	p.app.Press("id:computers-library-lib")
	p.app.WaitFor("id:library-film-f1", wait)
	p.app.Press("id:library-film-f1")
	p.app.WaitFor("id:film-download", wait)
	p.app.Press("id:film-download")
	p.app.WaitFor("id:film-play", wait)
	if !p.app.Exists("id:film-page") || p.app.Exists("id:downloads-page") {
		t.Fatal("a film downloaded while the computers show took the user away from them")
	}

	p.openTab(downloadsTab, msgs.DownloadsTitle())
	p.app.WaitFor("id:downloads-film-f1", wait)
	p.openTab(computersTab, msgs.ComputersTitle())
	p.app.WaitFor("id:film-page", wait)

	p.openTab(computersTab, msgs.ComputersTitle())
	p.app.WaitForGone("id:film-page", wait)
	p.app.WaitFor("id:computers-library-lib", wait)
	if p.app.Exists("id:library-page") {
		t.Fatal("pressing the tab shown again left the library's page on its stack")
	}
}

// With films on the phone and nothing paired, the phone opens on the downloads; the computers say there is none, pairing from there shows the computer paired, and the header's button pairs another.
func TestPairingFromTheComputersTab(t *testing.T) {
	d := newDesktop(t)
	dir := t.TempDir()
	keepTalk(t, dir)
	p := startPhone(t, dir)
	p.app.WaitFor("id:downloads-film-f1", wait)
	p.openTab(computersTab, msgs.ComputersTitle())
	p.app.WaitFor("id:computers-empty", wait)
	p.app.Press("id:computers-empty-pair")
	p.fillPairing(d.Addr.String(), "123456")
	p.app.Press("id:pair-continue")
	p.app.WaitFor("id:pair-compare", wait)
	p.app.Press("id:pair-confirm")
	p.app.WaitFor("id:computers-desktop-"+store.DesktopID(d.Fingerprint), wait)
	p.app.Press("id:computers-pair")
	p.app.WaitFor("id:pair-page", wait)
}

// Pairing again after the tabs gave way to the welcome screen starts each tab on its first page: a film's page left on the downloads' stack is of a film no longer kept.
func TestPairingAgainStartsTheTabsAfresh(t *testing.T) {
	d := newDesktop(t)
	p := startPhone(t, "")
	p.openTalk(d)
	d.Stall(300_000)
	p.app.Press("id:film-download")
	p.app.WaitFor("text:"+msgs.FilmDownloading(30), wait)
	p.app.Press("id:film-pause")
	p.app.WaitFor("text:"+msgs.FilmPaused(30), wait)

	// Unpaired, the computer leaves its paused download on the phone, which keeps the tabs.
	p.openTab(computersTab, msgs.ComputersTitle())
	id := store.DesktopID(d.Fingerprint)
	p.app.WaitFor("id:computers-options-"+id, wait)
	p.app.Press("id:computers-options-" + id)
	p.app.Answer(1)
	p.app.WaitFor("id:computers-empty", wait)
	p.openTab(downloadsTab, msgs.DownloadsTitle())
	p.app.WaitFor("id:downloads-film-f1", wait)
	p.app.Press("id:downloads-film-f1")
	p.app.WaitFor("id:film-cancel", wait)
	p.app.Press("id:film-cancel")
	p.app.WaitFor("id:welcome", wait)

	d.Stall(0)
	d.Offer("123456")
	p.pair(d)
	p.openTab(downloadsTab, msgs.DownloadsTitle())
	p.app.WaitFor("id:downloads-empty", wait)
	if p.app.Exists("id:film-page") {
		t.Fatal("the downloads tab shows the page of a film no longer kept")
	}
}

// On iOS a tab's root page puts its button in the system header, and a downloaded film's page its delete button.
func TestTheIOSHeaders(t *testing.T) {
	d := newDesktop(t)
	dir := t.TempDir()
	keepTalk(t, dir)
	p := startPhone(t, dir, pairedWith(t, d, dir), nsgotest.Environment(core.Environment{Locale: lang, OS: core.OSIOS}))
	p.app.WaitFor("id:downloads-film-f1", wait)
	p.app.Press("id:downloads-film-f1")
	p.app.WaitFor("id:film-delete", wait)
	p.app.HeaderBack("id:film-page")
	p.app.WaitForGone("id:film-page", wait)
	p.openTab(computersTab, msgs.ComputersTitle())
	p.app.WaitFor("id:computers-library-lib", wait)
	p.app.Press("id:computers-pair")
	p.app.WaitFor("id:pair-page", wait)
}

// Over a tab bar the app draws, which keeps clear of the bottom safe area itself, a film's actions sit just above it rather than a safe area higher.
func TestTheActionBarSitsOnTheTabBar(t *testing.T) {
	if nsgotest.Target() != "" {
		t.Skip("the safe area is the platform's on a target")
	}
	const bottom = 34
	dir := t.TempDir()
	keepTalk(t, dir)
	p := startPhone(t, dir, nsgotest.Environment(core.Environment{Locale: lang, SafeArea: core.Edges{Bottom: bottom}}))
	p.app.WaitFor("id:downloads-film-f1", wait)
	p.app.Press("id:downloads-film-f1")
	play := p.app.WaitFor("id:film-play", wait).Bounds()
	bar := p.app.Find("id:tab-bar").Bounds()
	if gap := bar.Origin.Y - (play.Origin.Y + play.Size.Height); gap < 0 || gap >= bottom {
		t.Fatalf("the action bar's button ends %v above the tab bar, want less than the %v of the safe area the bar keeps clear of", gap, core.Points(bottom))
	}
}
