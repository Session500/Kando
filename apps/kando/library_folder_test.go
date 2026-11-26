package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"kando/apps/kando/films"
	"kando/apps/kando/locales/testlocale"
	"kando/packages/core"
	"kando/packages/media"
)

// userFolderLibrary is a folder of the user's opened as a library: Show holds a video no Kando wrote beside (fresh), one an earlier Kando made the subtitles of beside it (old), with what it wrote there, and files of the user's own.
type userFolderLibrary struct {
	*homeFixture
	root, fresh, old string
}

func newUserFolderLibrary(t *testing.T) *userFolderLibrary {
	t.Helper()
	f := newHomeFixture(t)
	root := t.TempDir()
	show := filepath.Join(root, "Show")
	if err := os.Mkdir(show, 0o755); err != nil {
		t.Fatal(err)
	}
	fresh, old := filepath.Join(show, "lesson one.mp4"), filepath.Join(show, "old talk.mp4")
	for name, data := range map[string]string{
		"lesson one.mp4":         "v",
		"lesson one.en.srt":      "1\n00:00:00,000 --> 00:00:02,000\nthe user's own line\n\n",
		"old talk.mp4":           "vv",
		"old talk.bilingual.srt": homeOldBilingual,
		"notes.txt":              "mine",
	} {
		if err := os.WriteFile(filepath.Join(show, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := media.SaveCorrections(media.CorrectionsPath(old), []core.Correction{{Span: core.Span{End: time.Second}, Translation: ptr("您好")}}); err != nil {
		t.Fatal(err)
	}
	a := f.a
	a.edit(func(s *session) { s.Libraries = append(s.Libraries, root) })
	a.Choose(root)
	a.prefs.language = "简体中文"
	refresh(f.tt, a)
	l := &userFolderLibrary{homeFixture: f, root: root, fresh: fresh, old: old}
	l.before = folderHashes(t, root)
	return l
}

// wantFolderKept fails the test when the library's folder holds anything but what it held as the fixture was made, each file as it was, but for what Kando keeps in its hidden folders (media.FilesDir).
func (l *userFolderLibrary) wantFolderKept(t *testing.T, after string) {
	t.Helper()
	now := folderHashes(t, l.root)
	for name, sum := range l.before {
		if now[name] != sum {
			t.Errorf("after %s, %s in the user's folder changed or went", after, name)
		}
	}
	for name := range now {
		if _, ok := l.before[name]; !ok && !slices.Contains(strings.Split(name, string(filepath.Separator)), media.FilesDir) {
			t.Errorf("after %s, Kando wrote %s into the user's folder", after, name)
		}
	}
}

// A folder of the user's opened as a library has every file Kando writes of its films in the hidden folder beside each video (media.FolderHome): a video made, opened, corrected, given terms and chapters on the desktop and from a phone, and one an earlier Kando made beside, whose files are copied in as it opens, leave the user's files as they were, byte for byte. Renamed in Finder, the folder takes the films' files with it; removed, a film's folder goes with its video.
func TestAFolderLibraryWritesNothingBesideItsVideos(t *testing.T) {
	l := newUserFolderLibrary(t)
	a := l.a
	trashed := fakeTrash(t)

	// Made, its files are in its folder beside it.
	id, msg, _ := a.queueFromFolderChecked(homeFilm(t, a, l.fresh))
	if msg != "" {
		t.Fatal(msg)
	}
	waitUntil(t, l.tt, a, l.updates, func() bool {
		job, _ := a.library.Find(id)
		return job.Stage.Done() && a.queue.active == ""
	})
	job, _ := a.library.Find(id)
	home := media.FolderHome(l.fresh)
	if job.Stage != core.StageComplete || job.Home != home || job.SubtitlePath != filepath.Join(home, "video.bilingual.srt") || !fileExists(job.SubtitlePath) {
		t.Fatalf("the made film is %+v", job)
	}
	l.wantFolderKept(t, "making a film")
	l.writeAll(t, l.open(t, id))
	l.wantFolderKept(t, "writing a made film")
	a.closeFilm()

	// One an earlier Kando made beside its video has its files copied into its folder as it opens, and is written there.
	item := l.open(t, l.old)
	oldHome := media.FolderHome(l.old)
	if item.Home != oldHome || item.SubtitlePath != filepath.Join(oldHome, "video.bilingual.srt") {
		t.Fatalf("the earlier film opened is %+v", item)
	}
	if edits, _ := media.LoadCorrections(media.CorrectionsPath(item.SubtitlePath)); len(edits) != 1 {
		t.Fatalf("the earlier film's corrections copied are %v", edits)
	}
	l.writeAll(t, item)
	l.wantFolderKept(t, "writing an earlier film")
	a.closeFilm()
	// Listed again, it shows the subtitles in its folder, and is not copied again.
	a.refreshLibrary()
	if again := homeFilm(t, a, l.old); again.Home != oldHome || again.SubtitlePath != item.SubtitlePath {
		t.Fatalf("listed again, the earlier film is %+v", again)
	}

	// The show's folder renamed in Finder takes its films' files with it: a film whose job could not be paired with its video's new path is found there again, with its files.
	renamed := filepath.Join(l.root, "Show 2")
	if err := os.Rename(filepath.Dir(l.fresh), renamed); err != nil {
		t.Fatal(err)
	}
	a.refreshLibrary()
	video := filepath.Join(renamed, filepath.Base(l.fresh))
	for _, path := range []string{video, filepath.Join(renamed, filepath.Base(l.old))} {
		if moved := homeFilm(t, a, path); moved.Home != media.FolderHome(path) || !fileExists(moved.SubtitlePath) || filepath.Dir(moved.SubtitlePath) != moved.Home {
			t.Fatalf("after the rename the film is %+v", moved)
		}
	}
	gone := homeFilm(t, a, video)

	// Removed, a film's folder goes with its video, and the user's files beside it stay.
	a.TrashFilms(gone)
	refresh(l.tt, a)
	if !slices.Contains(*trashed, video) || !slices.Contains(*trashed, gone.Home) {
		t.Fatalf("trashed %v, want the video and its folder", *trashed)
	}
	if _, err := os.Stat(filepath.Join(renamed, "lesson one.en.srt")); err != nil {
		t.Fatalf("the user's own subtitles went with the film: %v", err)
	}
}

// A video put in the place of another under the same name, as another episode is in Finder, keeps the other's folder, which the player says once as it opens.
func TestAFolderLibrarySaysWhenAVideoIsAnotherFile(t *testing.T) {
	l := newUserFolderLibrary(t)
	a := l.a
	l.open(t, l.old)
	a.closeFilm()
	if err := os.WriteFile(l.old, []byte("another episode"), 0o644); err != nil {
		t.Fatal(err)
	}
	l.open(t, l.old)
	refresh(l.tt, a)
	name := films.Name(homeFilm(t, a, l.old))
	if got := a.lib.notice.Text; got != testlocale.Messages().LibraryHomeOtherVideo(name) {
		t.Fatalf("the message bar says %q", got)
	}
	a.closeFilm()
	a.lib.notice.Text = ""
	l.open(t, l.old)
	refresh(l.tt, a)
	if a.lib.notice.Text == testlocale.Messages().LibraryHomeOtherVideo(name) {
		t.Fatal("it is said again")
	}
}

// A film made in a folder of the user's opened as a library, moved to a library bundle, has its files beside its video there; moved back, they go into the folder beside it again, and nothing of Kando's is left where the user sees it.
func TestAFolderLibrarysFilmMovesToABundleAndBack(t *testing.T) {
	l := newUserFolderLibrary(t)
	a := l.a
	id, msg, _ := a.queueFromFolderChecked(homeFilm(t, a, l.fresh))
	if msg != "" {
		t.Fatal(msg)
	}
	waitUntil(t, l.tt, a, l.updates, func() bool {
		job, _ := a.library.Find(id)
		return job.Stage.Done() && a.queue.active == ""
	})
	bundle := filepath.Join(t.TempDir(), "other"+media.LibraryExt)
	if err := os.Mkdir(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	a.edit(func(s *session) { s.Libraries = append(s.Libraries, bundle) })

	a.MoveFilm(homeFilm(t, a, id), bundle)
	waitUntil(t, l.tt, a, l.updates, func() bool { return a.lib.move.run == nil })
	job, _ := a.library.Find(id)
	if job.LibraryDir != bundle || job.Home != "" || !films.InDir(bundle, job.VideoPath) || filepath.Dir(job.SubtitlePath) != filepath.Dir(job.VideoPath) || !fileExists(job.SubtitlePath) {
		t.Fatalf("moved to the bundle, the job is %+v", job)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(job.VideoPath), media.FilesDir)); err == nil {
		t.Fatal("the folder of its files came into the bundle")
	}
	if _, err := os.Stat(l.fresh); err == nil {
		t.Fatal("the video stayed in the folder it moved from")
	}
	if _, err := os.Stat(media.FolderHome(l.fresh)); err == nil {
		t.Fatal("the folder of its files stayed in the folder it moved from")
	}

	a.Choose(bundle)
	refresh(l.tt, a)
	a.MoveFilm(homeFilm(t, a, id), l.root)
	waitUntil(t, l.tt, a, l.updates, func() bool { return a.lib.move.run == nil })
	job, _ = a.library.Find(id)
	if job.LibraryDir != l.root || job.Home != media.FolderHome(job.VideoPath) || filepath.Dir(job.SubtitlePath) != job.Home || !fileExists(job.SubtitlePath) {
		t.Fatalf("moved back, the job is %+v", job)
	}
	entries, _ := os.ReadDir(filepath.Dir(job.VideoPath))
	for _, entry := range entries {
		if name := entry.Name(); name != filepath.Base(job.VideoPath) && name != media.FilesDir {
			t.Errorf("moved back, %s is beside the video", name)
		}
	}
}

// A film alone in its folder of a folder of the user's opened as a library moves to a library bundle with its whole folder, and the files in the folder beside its video come out of it beside the video there.
func TestAFolderLibrarysFilmAloneInItsFolderMovesToABundle(t *testing.T) {
	l := newUserFolderLibrary(t)
	a := l.a
	alone := filepath.Join(l.root, "Film", "film.mp4")
	if err := os.MkdirAll(filepath.Dir(alone), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(alone, []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.refreshLibrary()
	id, msg, _ := a.queueFromFolderChecked(homeFilm(t, a, alone))
	if msg != "" {
		t.Fatal(msg)
	}
	waitUntil(t, l.tt, a, l.updates, func() bool {
		job, _ := a.library.Find(id)
		return job.Stage.Done() && a.queue.active == ""
	})
	bundle := filepath.Join(t.TempDir(), "other"+media.LibraryExt)
	if err := os.Mkdir(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	a.edit(func(s *session) { s.Libraries = append(s.Libraries, bundle) })
	a.MoveFilm(homeFilm(t, a, id), bundle)
	waitUntil(t, l.tt, a, l.updates, func() bool { return a.lib.move.run == nil })
	job, _ := a.library.Find(id)
	if job.Home != "" || !films.InDir(bundle, job.VideoPath) || filepath.Dir(job.SubtitlePath) != filepath.Dir(job.VideoPath) || !fileExists(job.SubtitlePath) {
		t.Fatalf("moved to the bundle, the job is %+v", job)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(job.VideoPath), media.FilesDir)); err == nil {
		t.Fatal("the folder of its files came into the bundle")
	}
}

// A link downloaded into a folder of the user's opened as a library is given its folder beside its video as the video comes, before anything of it is written: its subtitles are made there, and nothing but the video is where the user sees it.
func TestALinkDownloadedIntoAFolderLibraryWritesIntoItsFolder(t *testing.T) {
	l := newUserFolderLibrary(t)
	a := l.a
	p := &fakePipeline{testFolder: testFolder{dir: filepath.Join(l.root, "Talk [abc12345]")}}
	if err := os.MkdirAll(p.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	a.pipeline = p
	l.before = folderHashes(t, l.root)
	if err := a.Enqueue("https://www.example.com/watch?v=one"); err != nil {
		t.Fatal(err)
	}
	var id string
	waitUntil(t, l.tt, a, l.updates, func() bool {
		for _, job := range a.library.Jobs {
			if job.Stage.Done() && a.queue.active == "" {
				id = job.ID
				return true
			}
		}
		return false
	})
	job, _ := a.library.Find(id)
	if job.Stage != core.StageComplete || job.Home != media.FolderHome(job.VideoPath) || filepath.Dir(job.SubtitlePath) != job.Home || !fileExists(job.SubtitlePath) {
		t.Fatalf("the link's job is %+v", job)
	}
	entries, _ := os.ReadDir(filepath.Dir(job.VideoPath))
	for _, entry := range entries {
		if name := entry.Name(); name != filepath.Base(job.VideoPath) && name != media.FilesDir {
			t.Errorf("%s is beside the downloaded video", name)
		}
	}
}
