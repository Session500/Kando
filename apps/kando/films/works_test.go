package films

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"kando/packages/core"
)

// workOf is the name and the films' keys of the work the film keyed key is in, "" for one in none.
func workOf(works WorkIndex, key string) (string, []string) {
	w, ok := works.Of(key)
	if !ok {
		return "", nil
	}
	return w.Name, w.Films
}

// listed is a done film of the list keyed list, at place, titled title when the list is one work.
func listed(key, list string, place int, title string) Film {
	return Film{Key: key, Source: "https://www.youtube.com/watch?v=" + key, Stage: core.StageComplete, List: list, ListIndex: place, ListTitle: title}
}

// Episodes whose names name their show fold into one work named after their folder, in the order they play; seasons in folders of their own are one work with them.
func TestWorksFoldAShowsEpisodes(t *testing.T) {
	all := []Film{
		local("/Videos/River Walk [BD]/River Walk S01E02.mkv"),
		local("/Videos/River Walk [BD]/River Walk S01E01.mkv"),
		local("/Videos/River Walk [BD]/trailer.mkv"),
		local("/Videos/Harbor/Season 2/02.mkv"),
		local("/Videos/Harbor/Season 1/01.mkv"),
		local("/Videos/Harbor/Season 1/02.mkv"),
		local("/Videos/Harbor/Season 2/01.mkv"),
	}
	works := Works(all, "/Library.kando", "en")
	if name, films := workOf(works, "River Walk S01E01.mkv"); name != "River Walk" || !slices.Equal(films, []string{"River Walk S01E01.mkv", "River Walk S01E02.mkv"}) {
		t.Fatalf("the show's work is %q %q", name, films)
	}
	if _, ok := works.Of("trailer.mkv"); ok {
		t.Fatal("a film whose name numbers nothing is in a work")
	}
	w, ok := works.Of("01.mkv")
	if !ok || w.Name != "Harbor" || len(w.Films) != 4 {
		t.Fatalf("the seasons' work is %+v", w)
	}
}

// Films that only sit side by side stay a card each: clips whose names start with numbers that are not episodes', episodes at the top of a library, scattered numbers, and two copies of one episode.
func TestWorksLeaveFilmsSideBySide(t *testing.T) {
	for name, all := range map[string][]Film{
		"clips":       {local("/Downloads/1. Introduction.mp4"), local("/Downloads/10 Tips.mp4"), local("/Downloads/3 Things.mp4")},
		"top":         {local("/Library/EP01.mp4"), local("/Library/EP02.mp4")},
		"scattered":   {local("/Downloads/EP1.mp4"), local("/Downloads/第12集.mp4")},
		"copies":      {local("/Talks/Lecture 1.mp4"), local("/Talks/Lecture 1 (1).mp4")},
		"unpadded":    {local("/Disc/1 Opening.mp4"), local("/Disc/2 Dialogue.mp4"), local("/Disc/3 Review.mp4")},
		"one episode": {local("/Videos/River Walk/River Walk EP01.mp4"), local("/Videos/River Walk/notes.mp4")},
		"channel":     {listed("a", "channel", 1, ""), listed("b", "channel", 2, "")},
	} {
		if works := Works(all, "/Library", "en"); len(works) != 0 {
			t.Errorf("%s: %d films are in works", name, len(works))
		}
	}
}

// Two courses that name their lessons alike, in folders side by side, are two works named after their folders; two shows in one folder are named after the shows; a course's sections named as courses name them take the course's name too.
func TestWorksNameEachWork(t *testing.T) {
	all := []Film{
		local("/Courses/Python/Lesson 01.mp4"), local("/Courses/Python/Lesson 02.mp4"),
		local("/Courses/Japanese/Lesson 01.mp4"), local("/Courses/Japanese/Lesson 02.mp4"),
		local("/Downloads/Harbor EP01.mp4"), local("/Downloads/Harbor EP02.mp4"),
		local("/Downloads/River Walk EP01.mp4"), local("/Downloads/River Walk EP02.mp4"),
		local("/Course/01 Getting Started/Part 1.mp4"), local("/Course/01 Getting Started/Part 2.mp4"),
	}
	for i := range all {
		all[i].Key = all[i].VideoPath
	}
	works := Works(all, "/Library.kando", "en")
	for key, want := range map[string]string{
		"/Courses/Python/Lesson 01.mp4":         "Python",
		"/Courses/Japanese/Lesson 02.mp4":       "Japanese",
		"/Downloads/Harbor EP01.mp4":            "Harbor",
		"/Downloads/River Walk EP02.mp4":        "River Walk",
		"/Course/01 Getting Started/Part 1.mp4": "Course · 01 Getting Started",
	} {
		if name, _ := workOf(works, key); name != want {
			t.Errorf("%s is in %q, want %q", key, name, want)
		}
	}
	if a, _ := works.Of("/Courses/Python/Lesson 01.mp4"); a == works["/Courses/Japanese/Lesson 01.mp4"] {
		t.Error("two courses are one work")
	}
}

// A playlist that is one work, as its title says, is one in its order; a fansub's releases fold as any show's.
func TestWorksFoldAListAndAFansubsReleases(t *testing.T) {
	all := []Film{
		listed("b", "list", 2, "【Playlist】 Cooking Basics"), listed("a", "list", 1, "【Playlist】 Cooking Basics"), listed("c", "list", 3, "Old name"),
		local("/Anime/山海旅人/[字幕组][山海旅人][02][1080P].mkv"), local("/Anime/山海旅人/[字幕组][山海旅人][01][1080P].mkv"),
	}
	works := Works(all, "/Library.kando", "zh-Hans")
	if name, films := workOf(works, "c"); name != "Cooking Basics" || !slices.Equal(films, []string{"a", "b", "c"}) {
		t.Fatalf("the playlist's work is %q %q", name, films)
	}
	if name, films := workOf(works, "[字幕组][山海旅人][01][1080P].mkv"); name != "山海旅人" || len(films) != 2 {
		t.Fatalf("the releases' work is %q %q", name, films)
	}
}

// A work is a run of films one after another in what Next plays, so opening its card and playing on plays its films in its order.
func TestWorksAreRunsOfASequence(t *testing.T) {
	all := []Film{
		local("/Courses/日语/第1节/日语课 第二讲.mp4"), local("/Courses/日语/第1节/日语课 第一讲.mp4"),
		local("/Courses/日语/第2节/日语课 第三讲.mp4"), local("/Courses/日语/第2节/日语课 第四讲.mp4"),
		local("/Videos/Harbor/Season 1/01.mkv"), local("/Videos/Harbor/Season 2/01.mkv"), local("/Videos/Harbor/Season 1/02.mkv"),
		local("/Videos/Harbor/Extras/01.mkv"), local("/Videos/Harbor/Extras/02.mkv"),
	}
	for i := range all {
		all[i].Key = all[i].VideoPath
	}
	works := Works(all, "/Library.kando", "zh-Hans")
	if len(works) == 0 {
		t.Fatal("no works")
	}
	for key, w := range works {
		item, _ := Find(all, key)
		seq := Keys(sequence(all, item, "zh-Hans"))
		at := slices.Index(seq, w.Films[0])
		if at < 0 || at+len(w.Films) > len(seq) || !slices.Equal(seq[at:at+len(w.Films)], w.Films) {
			t.Fatalf("work %q %q is not a run of its sequence %q", w.Name, w.Films, seq)
		}
	}
}

// A large library's works are had at once: 2000 films, 300 shows of them, in about 5 ms on the machine this was written on, and about ten times that on a slow test machine. The library works them out again only when its films change (WorksCache), so what the test keeps out is a way of working them out that grows with the square of the films, as reading each film's sequence apart did (2 s for these).
func TestWorksOfALargeLibrary(t *testing.T) {
	all := largeLibrary()
	// The quickest of three, so a test machine busy with others does not count.
	var works WorkIndex
	took := time.Hour
	for range 3 {
		start := time.Now()
		works = Works(all, "/Library.kando", "en")
		took = min(took, time.Since(start))
	}
	// The race detector makes every step many times slower, so only what Works finds is checked under it.
	if took > 250*time.Millisecond && !raceOn {
		t.Fatalf("2000 films' works took %v", took)
	}
	if len(works) < 1500 {
		t.Fatalf("only %d of 2000 films are in works", len(works))
	}
}

func BenchmarkWorks(b *testing.B) {
	all := largeLibrary()
	for b.Loop() {
		Works(all, "/Library.kando", "en")
	}
}

// largeLibrary is 2000 local films: 300 shows of 6 episodes, two seasons each, and 200 films alone.
func largeLibrary() []Film {
	var all []Film
	for s := range 300 {
		for e := range 6 {
			all = append(all, local(fmt.Sprintf("/Videos/Show %d/Season %d/Show %d S%02dE%02d.mkv", s, e/3+1, s, e/3+1, e%3+1)))
		}
	}
	for i := range 200 {
		all = append(all, local(filepath.Join("/Downloads", fmt.Sprintf("Clip about rivers %c%c.mp4", 'a'+i%26, 'a'+i/26))))
	}
	for i := range all {
		all[i].Key = all[i].VideoPath
	}
	return all
}
