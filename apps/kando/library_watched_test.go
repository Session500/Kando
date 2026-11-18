package main

import (
	"path/filepath"
	"testing"
	"time"

	"kando/apps/kando/films"
	"kando/apps/kando/sidebar"
	"kando/packages/core"
)

// TestMarkingAFilmWatchedSurvivesARestart checks that 标记为已看完 has a film played partway count as played to its end, forgetting where it stopped but not when it played, and lets the palette find every line of it; that 标记为未看 forgets all three, so it shows as never played and the palette finds none of its lines; that both are saved with the library at once; and that a film whose length is not known is not marked watched.
func TestMarkingAFilmWatchedSurvivesARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	video := "/videos/talk/video.mp4"
	a, _, _ := newTestApp(t, &fakePipeline{})
	a.openLibrary(path)
	a.library.Jobs = []core.Job{{ID: "talk", Source: core.FileURL(video), Stage: core.StageComplete, VideoPath: video}}
	item, _ := films.Find(a.films(), "talk")
	a.paletteNoteFurthest(video, 40*time.Second)
	a.rememberPosition(&filmPlay{path: video, loaded: true, duration: 100 * time.Second, head: 40 * time.Second})
	played := a.PlayedAt(video)
	if played.IsZero() || a.ResumeAt(video) != 40*time.Second {
		t.Fatalf("the film did not play partway: stopped at %v, played at %v", a.ResumeAt(video), played)
	}

	// Its length is not known yet: there is nothing to mark it watched to.
	a.MarkWatched(item)
	if a.Furthest(video) != 40*time.Second || a.ResumeAt(video) != 40*time.Second {
		t.Fatalf("a film of unknown length marked watched: furthest %v, stopped at %v", a.Furthest(video), a.ResumeAt(video))
	}

	a.lib.durations = map[string]time.Duration{video: 100 * time.Second}
	a.MarkWatched(item)
	reopen := func() *app {
		t.Helper()
		again, _, _ := newTestApp(t, &fakePipeline{})
		again.openLibrary(path)
		again.lib.durations = map[string]time.Duration{video: 100 * time.Second}
		return again
	}
	for _, b := range []*app{a, reopen()} {
		if got := films.WatchState(b, item); got != sidebar.WatchWatched {
			t.Fatalf("marked watched, the film is %q", got)
		}
		if b.Furthest(video) != 100*time.Second || b.ResumeAt(video) != 0 || !b.PlayedAt(video).Equal(played) {
			t.Fatalf("marked watched: furthest %v, stopped at %v, played at %v (was %v)", b.Furthest(video), b.ResumeAt(video), b.PlayedAt(video), played)
		}
		if got := b.paletteHorizon(item); got != 100*time.Second {
			t.Fatalf("the palette finds the lines up to %v of a film marked watched", got)
		}
	}

	a.MarkUnwatched(item)
	for _, b := range []*app{a, reopen()} {
		if got := films.WatchState(b, item); got != sidebar.WatchUnwatched {
			t.Fatalf("marked unwatched, the film is %q", got)
		}
		if b.Furthest(video) != 0 || b.ResumeAt(video) != 0 || !b.PlayedAt(video).IsZero() {
			t.Fatalf("marked unwatched: furthest %v, stopped at %v, played at %v", b.Furthest(video), b.ResumeAt(video), b.PlayedAt(video))
		}
		if _, ok := b.library.Furthest[video]; ok {
			t.Fatal("library.json still says how far a film marked unwatched played")
		}
	}
}

// TestPlayerTellsAFilmsLength checks that the library learns a video's length from the player when the poster's probe has not told it, as where ffmpeg is missing, so its check can show and it can be marked watched, and that a length already known stays.
func TestPlayerTellsAFilmsLength(t *testing.T) {
	a, tt, updates, item := playerFilm(t, 3, nil)
	delete(a.lib.durations, item.VideoPath)
	playerOpenFilm(t, a, tt, updates, item)
	playerLoadFilm(tt, a, 90*time.Second)
	if got := a.Duration(item.VideoPath); got != 90*time.Second {
		t.Fatalf("the player's length is %v in the library", got)
	}
	a.libraryLearnDuration(item.VideoPath, 60*time.Second)
	if got := a.Duration(item.VideoPath); got != 90*time.Second {
		t.Fatalf("a length known already became %v", got)
	}
}
