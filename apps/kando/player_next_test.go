package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"kando/apps/kando/films"
	"kando/apps/kando/playlist"
	"kando/apps/kando/subscribe"
	"kando/packages/core"
	"kando/packages/media"

	"github.com/nsgo-dev/nsgo/nsgotest"
)

// nextCourse is a library holding a course of three lessons in a folder of their own, each a local video with its subtitles as a finished job made them, the third still waiting to be made; it returns the app showing the library, its window, its background updates and the lessons in order.
func nextCourse(t *testing.T) (*app, *nsgotest.App, chan func(), []films.Film) {
	t.Helper()
	dir := t.TempDir()
	course := filepath.Join(dir, "Course")
	var jobs []core.Job
	for i, name := range []string{"Lesson 01", "Lesson 02", "Lesson 03"} {
		video := filepath.Join(course, name+".mp4")
		writeFakeClip(t, video, 12, "202020", "101010")
		srt := filepath.Join(course, name+".bilingual.srt")
		if err := os.WriteFile(srt, []byte(talkSubtitles), 0o644); err != nil {
			t.Fatal(err)
		}
		job := core.Job{ID: []string{"l1", "l2", "l3"}[i], Title: name, Source: core.FileURL(video), Language: "简体中文", Stage: core.StageComplete, VideoPath: video, SubtitlePath: srt, LibraryDir: dir}
		if i == 2 {
			job.Stage, job.SubtitlePath = core.StageQueued, ""
		}
		jobs = append([]core.Job{job}, jobs...)
	}
	a, tt, updates := newTestApp(t, &fakePipeline{})
	a.player.remuxDir = filepath.Join(t.TempDir(), "playback")
	a.settingsPath = filepath.Join(t.TempDir(), "settings.json")
	a.queue.paused = true
	a.edit(func(s *session) { s.LibraryDir = dir })
	a.library.Jobs = jobs
	refresh(tt, a)
	var lessons []films.Film
	for _, id := range []string{"l1", "l2", "l3"} {
		item, ok := films.Find(a.films(), id)
		if !ok {
			t.Fatalf("lesson %s is not on the shelf: %+v", id, a.films())
		}
		lessons = append(lessons, item)
	}
	return a, tt, updates, lessons
}

// nextEnd opens item, has the system's player load it and play it to its end.
func nextEnd(t *testing.T, a *app, tt *nsgotest.App, updates chan func(), item films.Film) {
	t.Helper()
	playerOpenFilm(t, a, tt, updates, item)
	playerLoadFilm(tt, a, 12*time.Second)
	tt.Progress("id:film-video", 12*time.Second, 12*time.Second)
	refresh(tt, a)
}

// A lesson played to its end shows the next one on a card that counts down and then plays it from its start; the card does not come back once put away, and Space at the end plays the lesson again rather than the next.
func TestTheNextLessonPlaysOnceOneEnds(t *testing.T) {
	a, tt, updates, lessons := nextCourse(t)
	nextEnd(t, a, tt, updates, lessons[0])
	if !tt.Exists("id:film-next") || tt.Find("id:film-next-title").Text != "Lesson 02" {
		t.Fatalf("the card shows %v %q", tt.Exists("id:film-next"), tt.Texts())
	}
	tt.Advance(nextAfter)
	refresh(tt, a)
	if a.openKey != "l2" || tt.Exists("id:film-next") {
		t.Fatalf("the countdown opened %q", a.openKey)
	}
	if !a.player.playJump && a.player.play != nil && a.player.play.paused {
		t.Fatal("the next lesson opened held")
	}

	// Escape puts the card away, and it stays away while the lesson stays at its end.
	playerLoadFilm(tt, a, 12*time.Second)
	tt.Progress("id:film-video", 12*time.Second, 12*time.Second)
	refresh(tt, a)
	if !tt.Exists("id:film-next") {
		t.Fatal("no card after the second lesson")
	}
	tt.Key("escape")
	a.flash("something else happened")
	refresh(tt, a)
	if tt.Exists("id:film-next") || a.openKey != "l2" {
		t.Fatalf("the card came back, or Escape left the film (%q)", a.openKey)
	}
}

// The next lesson still waiting to be made goes first in the queue once the lesson before it is opened, and plays once that ends, made as it plays, its video being there already.
func TestTheNextLessonGoesFirst(t *testing.T) {
	a, tt, updates, lessons := nextCourse(t)
	a.queue.order = []string{"other", "l3"}
	nextEnd(t, a, tt, updates, lessons[1])
	if len(a.queue.order) == 0 || a.queue.order[0] != "l3" {
		t.Fatalf("queue %v, want the next lesson first", a.queue.order)
	}
	if !tt.Exists("id:film-next") || tt.Find("id:film-next-title").Text != "Lesson 03" || a.player.next.due.IsZero() {
		t.Fatalf("the card shows %q, due %v", tt.Texts(), a.player.next.due)
	}
}

// A playlist's next video not downloaded yet is waited for on the card, without a countdown, which starts once its video is there.
func TestTheNextVideoOfAListIsWaitedFor(t *testing.T) {
	a, tt, updates, item := playerFilm(t, 12, nil)
	list := "youtube.com/playlist?list=PL1"
	first, _ := a.library.Find("talk")
	first.List, first.ListIndex = list, 1
	a.library.Upsert(first)
	second := core.Job{ID: "e2", Title: "Episode 2", Source: "https://www.youtube.com/watch?v=e2", Language: "简体中文", Stage: core.StageQueued, LibraryDir: a.snap().LibraryDir, List: list, ListIndex: 2}
	a.library.Upsert(second)
	a.queue.paused = true
	refresh(tt, a)
	item, _ = films.Find(a.films(), item.Key)
	nextEnd(t, a, tt, updates, item)
	if !tt.Exists("id:film-next") || tt.Find("id:film-next-title").Text != "Episode 2" || !a.player.next.due.IsZero() {
		t.Fatalf("the card shows %q, due %v", tt.Texts(), a.player.next.due)
	}
	tt.Advance(nextAfter)
	refresh(tt, a)
	if a.openKey != "talk" {
		t.Fatalf("a video not downloaded yet opened: %q", a.openKey)
	}
	// Its video comes: the countdown starts.
	second.Stage, second.VideoPath = core.StageTranscribing, filepath.Join(a.snap().LibraryDir, "Episode 2 [e2]", "video.mp4")
	writeFakeClip(t, second.VideoPath, 12, "202020", "101010")
	apply(tt, a, func() { a.library.Upsert(second) })
	if a.player.next.due.IsZero() {
		t.Fatal("the countdown did not start once the video came")
	}
}

// With 自动播放下一部 off no card shows, and the setting is kept; ⇧N plays the next lesson all the same, and ⇧P goes back to the one before.
func TestPlayingNextCanBeTurnedOff(t *testing.T) {
	a, tt, updates, lessons := nextCourse(t)
	apply(tt, a, a.ToggleAutoNext)
	saved, err := os.ReadFile(a.settingsPath)
	if err != nil || !strings.Contains(string(saved), `"playNext": false`) {
		t.Fatalf("settings %s, %v", saved, err)
	}
	nextEnd(t, a, tt, updates, lessons[0])
	if tt.Exists("id:film-next") {
		t.Fatal("the card shows with 自动播放下一部 off")
	}
	tt.Key("shift+n")
	refresh(tt, a)
	if a.openKey != "l2" {
		t.Fatalf("⇧N opened %q", a.openKey)
	}
	tt.Key("shift+p")
	refresh(tt, a)
	if a.openKey != "l1" {
		t.Fatalf("⇧P opened %q", a.openKey)
	}
}

// A line looped at the film's end goes back to the line's start rather than on to the next film.
func TestALoopedLineKeepsTheFilm(t *testing.T) {
	a, tt, updates, lessons := nextCourse(t)
	playerOpenFilm(t, a, tt, updates, lessons[0])
	playerLoadFilm(tt, a, 12*time.Second)
	apply(tt, a, a.ToggleLoopLine)
	tt.Progress("id:film-video", 12*time.Second, 12*time.Second)
	refresh(tt, a)
	if tt.Exists("id:film-next") {
		t.Fatal("a looped line went on to the next film")
	}
}

// A playlist read by the dialog places its films in its own order, and read again in its new one; a channel places them from its oldest, its new videos after those placed already; a subscription places what it adds after those.
func TestListsPlaceTheirFilms(t *testing.T) {
	a, tt, _ := newTestApp(t, &fakePipeline{})
	dir := t.TempDir()
	a.settingsPath = filepath.Join(t.TempDir(), "settings.json")
	a.edit(func(s *session) { s.LibraryDir = dir })
	job := func(id string) core.Job {
		return core.Job{ID: id, Source: "https://www.youtube.com/watch?v=" + id, Stage: core.StageQueued, LibraryDir: dir}
	}
	for _, id := range []string{"a", "b", "c", "n1", "n2", "n3", "n4"} {
		a.library.Upsert(job(id))
	}
	refresh(tt, a)
	listed := func(ids ...string) media.Listing {
		var l media.Listing
		for _, id := range ids {
			l.Videos = append(l.Videos, media.ListedVideo{URL: "https://www.youtube.com/watch?v=" + id})
		}
		return l
	}
	places := func(ids ...string) []int {
		var out []int
		for _, id := range ids {
			j, _ := a.library.Find(id)
			out = append(out, j.ListIndex)
		}
		return out
	}
	playlistSource := "https://www.youtube.com/playlist?list=PL1"
	a.nextKeepLists([]playlist.List{{Source: playlistSource, Listing: listed("b", "a", "c")}})
	if got := places("b", "a", "c"); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("playlist places %v", got)
	}
	a.nextKeepLists([]playlist.List{{Source: playlistSource, Listing: listed("a", "b", "c")}})
	if got := places("a", "b", "c"); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("playlist read again places %v", got)
	}
	channel := "https://www.youtube.com/@river"
	a.nextKeepLists([]playlist.List{{Source: channel, Channel: true, Listing: listed("n2", "n1")}})
	if got := places("n1", "n2"); !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("channel places %v", got)
	}
	a.nextKeepLists([]playlist.List{{Source: channel, Channel: true, Listing: listed("n3", "n2")}})
	if got := places("n1", "n2", "n3"); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("channel read again places %v", got)
	}
	sub := &subscribe.Sub{ID: "s1", Source: channel, Library: dir}
	a.nextKeepSubscribed(sub, []subscribe.Video{{URL: "https://www.youtube.com/watch?v=n4"}})
	if got := places("n4"); !slices.Equal(got, []int{4}) {
		t.Fatalf("subscription places %v", got)
	}
	if j, _ := a.library.Find("n4"); j.List != queueSourceKey(channel) {
		t.Fatalf("the subscription's video is in list %q", j.List)
	}
}
