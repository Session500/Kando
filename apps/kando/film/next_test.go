package film

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// fakeNexter is the card that plays the next film, up, and what was asked of it.
type fakeNexter struct {
	up              UpNext
	played, dropped int
}

func (f *fakeNexter) UpNext() UpNext { return f.up }
func (f *fakeNexter) PlayNext()      { f.played++; f.up = UpNext{} }
func (f *fakeNexter) CancelNext()    { f.dropped++; f.up = UpNext{} }
func (f *fakeNexter) HasNext() bool  { return f.up.Open }

// The card names the next film and counts down to it a second at a time; Return plays it, and the transport offers it.
func TestNextCardCountsDownAndPlays(t *testing.T) {
	a := newFakeApp()
	app := start(t, a)
	if app.Exists("id:film-next") || app.Exists("id:film-next-film") {
		t.Fatal("the card shows with no film to play next")
	}
	change(app, a, func() {
		a.fakeNexter.up = UpNext{Open: true, Title: "Lesson 2", Due: app.Clock.Now().Add(8 * time.Second), Countdown: 8 * time.Second}
	})
	if got := app.Find("id:film-next-title").Text; got != "Lesson 2" {
		t.Fatalf("the card names %q", got)
	}
	if got := app.Find("id:film-next-status").Text; got != "8 秒后播放" {
		t.Fatalf("the card says %q", got)
	}
	app.Advance(3 * time.Second)
	if got := app.Find("id:film-next-status").Text; got != "5 秒后播放" {
		t.Fatalf("three seconds on the card says %q", got)
	}
	if !app.Exists("id:film-next-film") {
		t.Fatal("the transport offers no next film")
	}
	app.Key("enter")
	if a.fakeNexter.played != 1 || app.Exists("id:film-next") {
		t.Fatalf("Return played %d", a.fakeNexter.played)
	}
}

// While the next film is still being made the card says how far, and Escape puts it away.
func TestNextCardWaitsForTheNextFilm(t *testing.T) {
	a := newFakeApp()
	app := start(t, a)
	change(app, a, func() {
		a.fakeNexter.up = UpNext{Open: true, Title: "Lesson 3", Waiting: func(*core.Scope) string { return "排队中" }}
	})
	if got := app.Find("id:film-next-status").Text; got != "排队中" {
		t.Fatalf("the card says %q", got)
	}
	app.Key("escape")
	if a.fakeNexter.dropped != 1 {
		t.Fatalf("Escape put the card away %d times", a.fakeNexter.dropped)
	}
}
