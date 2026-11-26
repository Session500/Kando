package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"kando/apps/kando/films"
	"kando/packages/core"
)

// worksLibrary is an open library of two courses' lessons kept where they were picked, Python's three and Japanese's two, done but for Python's third, which waits.
func worksLibrary(t *testing.T) (a *app, jobs []core.Job) {
	t.Helper()
	lib := filepath.Join(t.TempDir(), "课程.kando")
	from := t.TempDir()
	a, tt, _ := newTestApp(t, &fakePipeline{})
	for i, path := range []string{"Python/Lesson 01.mp4", "Python/Lesson 02.mp4", "Python/Lesson 03.mp4", "Japanese/Lesson 01.mp4", "Japanese/Lesson 02.mp4"} {
		video := filepath.Join(from, "Courses", filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(video), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(video, []byte("v"), 0o644); err != nil {
			t.Fatal(err)
		}
		stage := core.StageComplete
		if i == 2 {
			stage = core.StageQueued
		}
		jobs = append(jobs, core.Job{ID: []string{"p1", "p2", "p3", "j1", "j2"}[i], Source: core.FileURL(video), Language: "简体中文", Stage: stage, VideoPath: video, LibraryDir: lib})
	}
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	apply(tt, a, func() {
		a.library.Jobs = slices.Clone(jobs)
		a.switchLibrary(lib)
	})
	return a, jobs
}

// A course's new lesson goes into the categories every lesson before it is in, and not into one only some of them are.
func TestANewEpisodeJoinsItsWorksCategories(t *testing.T) {
	a, jobs := worksLibrary(t)
	a.categoryPutNamed("Python", jobs[0].VideoPath, jobs[1].VideoPath)
	a.categoryPutNamed("难点", jobs[1].VideoPath)
	python, _ := a.cats.book.Named("Python")
	hard, _ := a.cats.book.Named("难点")
	done := jobs[2]
	done.Stage = core.StageComplete
	a.show(done)
	if !a.cats.book.In(done.VideoPath, python.ID) || a.cats.book.In(done.VideoPath, hard.ID) {
		t.Fatalf("the new lesson is in %v", a.cats.book.Of(done.VideoPath))
	}
	// Japanese's lesson, of another work, stays in none.
	if got := a.cats.book.Of(jobs[3].VideoPath); len(got) != 0 {
		t.Fatalf("another course's lesson is in %v", got)
	}
}

// The Up Next card names the work play goes on into when it is not the open film's.
func TestUpNextNamesTheWorkItLeavesFor(t *testing.T) {
	a, jobs := worksLibrary(t)
	a.openKey = "p2"
	for _, job := range jobs {
		job.Stage = core.StageComplete
		a.library.Upsert(job)
	}
	items := a.films()
	same, _ := films.Find(items, "p3")
	other, _ := films.Find(items, "j1")
	if got := a.nextTitle(same); got != "Lesson 03" {
		t.Fatalf("the next lesson of the same course is called %q", got)
	}
	if got := a.nextTitle(other); got != a.messages().FilmNextInWork("Japanese", "Lesson 01") {
		t.Fatalf("a lesson of another course is called %q", got)
	}
}
