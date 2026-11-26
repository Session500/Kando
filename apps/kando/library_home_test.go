package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"kando/apps/kando/films"
	"kando/apps/kando/locales/testlocale"
	"kando/apps/kando/phonesync"
	"kando/apps/kando/shelf"
	"kando/packages/core"
	"kando/packages/media"
	psync "kando/packages/sync"
	"kando/packages/track"

	"github.com/nsgo-dev/nsgo/nsgotest"
)

// localPipeline is fakePipeline for videos picked on disk: Download of a file:// source is the file itself, where the user keeps it, as media.Tools' is, so the film's files are written where Kando writes them for a video outside the library.
type localPipeline struct{ fakePipeline }

func (p *localPipeline) Download(ctx context.Context, source string) (string, error) {
	if u, err := url.Parse(source); err == nil {
		if path, ok := core.FilePath(u); ok {
			return path, nil
		}
	}
	return p.fakePipeline.Download(ctx, source)
}

// homeFixture is a library bundle of the test's own, testVideos, and a folder of the user's outside it, user, holding a video no Kando has written beside (fresh), one an earlier Kando made the subtitles of beside it (old), with what it wrote there, and files of the user's own, among them subtitles of the user's that share the old video's name.
type homeFixture struct {
	a       *app
	tt      *nsgotest.App
	updates chan func()
	user    string
	fresh   string
	old     string
	// before is what user held as the fixture was made, each file's name with the hash of what it held.
	before map[string]string
}

// The subtitles an earlier Kando wrote beside old: its transcript, its bilingual subtitles, the user's corrections of them and its terms, and the user's own subtitles of the same name, which an earlier Kando kept its transcript aside from.
const (
	homeOldLine      = "1\n00:00:00,000 --> 00:00:01,000\nhello\n\n"
	homeOldBilingual = "1\n00:00:00,000 --> 00:00:01,000\n你好\nhello\n\n"
)

func newHomeFixture(t *testing.T) *homeFixture {
	t.Helper()
	user := t.TempDir()
	fresh, old := filepath.Join(user, "lesson one.mp4"), filepath.Join(user, "old talk.mp4")
	for name, data := range map[string]string{
		"lesson one.mp4":         "v",
		"old talk.mp4":           "v",
		"old talk.kando.srt":     homeOldLine,
		"old talk.srt":           "1\n00:00:00,000 --> 00:00:02,000\nthe user's own line\n\n",
		"old talk.bilingual.srt": homeOldBilingual,
		"notes.txt":              "mine",
	} {
		if err := os.WriteFile(filepath.Join(user, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := media.SaveCorrections(media.CorrectionsPath(old), []core.Correction{{Span: core.Span{End: time.Second}, Translation: ptr("您好")}}); err != nil {
		t.Fatal(err)
	}
	if err := media.SaveTerms(media.TermsPath(old), []core.Term{{Source: "hello", Target: "你好", Scope: core.ScopeFilm, Origin: core.OriginUser}}); err != nil {
		t.Fatal(err)
	}
	a, tt, updates := newTestApp(t, &localPipeline{})
	f := &homeFixture{a: a, tt: tt, updates: updates, user: user, fresh: fresh, old: old}
	f.before = folderHashes(t, user)
	return f
}

// folderHashes is every file under dir, by its path in dir, with the hash of what it holds.
func folderHashes(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		sum := sha256.Sum256(data)
		out[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// wantUserFolderKept fails the test when the user's folder holds anything but what it held as the fixture was made, each file as it was.
func (f *homeFixture) wantUserFolderKept(t *testing.T, after string) {
	t.Helper()
	now := folderHashes(t, f.user)
	for name, sum := range f.before {
		if now[name] != sum {
			t.Errorf("after %s, %s in the user's folder changed or went", after, name)
		}
	}
	for name := range now {
		if _, ok := f.before[name]; !ok {
			t.Errorf("after %s, Kando wrote %s into the user's folder", after, name)
		}
	}
}

// queue makes the subtitles of the video at video as dropping it does, and waits for its job to complete.
func (f *homeFixture) queue(t *testing.T, video string) core.Job {
	t.Helper()
	id, err := f.a.queueTake(core.FileURL(video))
	if err != nil {
		t.Fatal(err)
	}
	waitUntil(t, f.tt, f.a, f.updates, func() bool {
		job, _ := f.a.library.Find(id)
		return job.Stage.Done() && f.a.queue.active == ""
	})
	job, _ := f.a.library.Find(id)
	if job.Stage != core.StageComplete {
		t.Fatalf("the job of %s ended %s: %v", filepath.Base(video), job.Stage, job.Err)
	}
	return job
}

// open opens the film of key in the player and waits for its lines.
func (f *homeFixture) open(t *testing.T, key string) films.Film {
	t.Helper()
	f.a.Open(homeFilm(t, f.a, key))
	waitUntil(t, f.tt, f.a, f.updates, func() bool { return f.a.correctLoaded() && f.a.gloss.filmRead })
	item, ok := f.a.openFilm()
	if !ok {
		t.Fatal("no film open")
	}
	return item
}

// writeAll writes the open film's files as each of the player's tabs and a paired phone do: a correction of its first line, a term of its own, its chapter cut, a correction and a term from a phone.
func (f *homeFixture) writeAll(t *testing.T, item films.Film) {
	t.Helper()
	a := f.a
	if !a.correctWrite([]core.Correction{{Span: core.Span{End: time.Second}, Translation: ptr("改过的")}}) {
		t.Fatal("the correction was not written")
	}
	if !a.glossaryPut(core.Term{Source: "lesson", Target: "课", Scope: core.ScopeFilm, Origin: core.OriginUser}) {
		t.Fatal("the term was not written")
	}
	// Chapters kept with the film's files, as a site's or its file's are (media.KeepEmbeddedChapters), named after what the player names them after: a video kept outside the library has them in its folder there.
	chapters := []core.Chapter{{Start: 0, Title: "one"}, {Start: 10 * time.Second, Title: "two"}}
	if err := media.SaveChapters(a.player.film.infoBase, media.FilmChapters{Chapters: chapters}); err != nil {
		t.Fatal(err)
	}
	a.player.film.info.Chapters = chapters
	a.SetFilmChapterUse(core.ChaptersEven)
	if kept, _ := media.ReadChapters(a.player.film.infoBase); kept.Use != core.ChaptersEven {
		t.Fatalf("the chapter cut kept is %q", kept.Use)
	}
	dir := a.snap().LibraryDir
	phoneFilm := phoneFilmOf(item, phoneShelf{})
	if _, err := a.phoneStateCorrect(dir, phoneFilm, homePhoneEdit(phoneFilm.ID, "手机改的"), nil, false); err != nil {
		t.Fatalf("a phone's corrections were not written: %v", err)
	}
	if _, err := a.phoneGlossaryPut(dir, item, core.Term{Source: "phone", Target: "手机", Scope: core.ScopeFilm, Origin: core.OriginUser}); err != nil {
		t.Fatalf("a phone's term was not written: %v", err)
	}
	refresh(f.tt, a)
}

// A video kept outside the library, made, opened, corrected, given terms and chapters on the desktop and from a phone, made again, has every file Kando writes of it in its folder in the library, named as a film the library holds, and the user's folder is left as it was, byte for byte; so is an earlier Kando's film of a video there, whose files are copied into its folder as it opens, the user's own subtitles of its name left out.
func TestOutsideFilmsWriteIntoTheLibraryAlone(t *testing.T) {
	f := newHomeFixture(t)
	a := f.a

	// A new film.
	job := f.queue(t, f.fresh)
	f.wantUserFolderKept(t, "making a new film")
	want := shelf.HomeFor(testVideos, f.fresh, job.ID)
	if job.Home != want || job.VideoPath != f.fresh {
		t.Fatalf("the job's folder is %q and video %q, want %q and the video where it is", job.Home, job.VideoPath, want)
	}
	if job.SubtitlePath != filepath.Join(want, "video.bilingual.srt") || !fileExists(job.SubtitlePath) {
		t.Fatalf("the subtitles are at %q", job.SubtitlePath)
	}
	if logs := track.Logs(job.FileBase()); len(logs) != 1 {
		t.Fatalf("the film's logs in its folder are %v", logs)
	}
	// The library read again lists the film once, by its job: its folder in the library is no film of its own.
	a.refreshLibrary()
	if items := a.films(); len(items) != 1 || items[0].JobID != job.ID {
		t.Fatalf("the library lists %+v", items)
	}
	item := f.open(t, job.ID)
	f.writeAll(t, item)
	f.wantUserFolderKept(t, "writing a new film")
	for _, name := range []string{".video.edits.json", ".video.terms.json", ".video.chapters.json"} {
		if !fileExists(filepath.Join(want, name)) {
			t.Errorf("%s is not in the film's folder", name)
		}
	}
	// Made again, translated again and its missing lines filled.
	for _, redo := range []func(){
		func() { a.retry(job) },
		func() { _ = a.queueRetranslate(job, core.RedoTranslation, "") },
		func() { _ = a.queueRetranslate(job, core.RedoMissing, "") },
	} {
		redo()
		waitUntil(t, f.tt, a, f.updates, func() bool {
			j, _ := a.library.Find(job.ID)
			return j.Stage.Done() && a.queue.active == ""
		})
		job, _ = a.library.Find(job.ID)
	}
	f.wantUserFolderKept(t, "making a film again")
	if job.Home != want || filepath.Dir(job.SubtitlePath) != want {
		t.Fatalf("made again, the job's folder is %q and subtitles %q", job.Home, job.SubtitlePath)
	}

	// An earlier Kando's film, its subtitles beside its video.
	a.library.Jobs = append(a.library.Jobs, core.Job{ID: "0123456789abcdef", Source: core.FileURL(f.old), Language: "简体中文", Stage: core.StageComplete, CompletedAt: time.Now(), VideoPath: f.old, SubtitlePath: media.BilingualPath(f.old), LibraryDir: testVideos})
	refresh(f.tt, a)
	item = f.open(t, "0123456789abcdef")
	old, _ := a.library.Find("0123456789abcdef")
	home := shelf.HomeFor(testVideos, f.old, old.ID)
	if old.Home != home || item.Home != home || old.SubtitlePath != filepath.Join(home, "video.bilingual.srt") {
		t.Fatalf("the earlier film's folder is %q, its subtitles %q", old.Home, old.SubtitlePath)
	}
	// The player read its lines from its folder the first time it read them: the film open is never one whose files move under it.
	if a.player.subs.path != old.SubtitlePath || a.player.subs.base != old.FileBase() {
		t.Fatalf("the player reads %q named after %q", a.player.subs.path, a.player.subs.base)
	}
	for name, want := range map[string]string{"video.srt": homeOldLine, "video.bilingual.srt": homeOldBilingual} {
		if data, _ := os.ReadFile(filepath.Join(home, name)); string(data) != want {
			t.Errorf("%s in the film's folder holds %q", name, data)
		}
	}
	if fileExists(filepath.Join(home, "video.kando.srt")) {
		t.Error("the transcript kept aside beside the video is kept aside in the film's folder too")
	}
	if edits, _ := media.LoadCorrections(filepath.Join(home, ".video.edits.json")); len(edits) != 1 {
		t.Errorf("the earlier film's corrections are %v in its folder", edits)
	}
	if a.lib.notice.Text != testlocale.Messages().LibraryAdopted(films.Name(item)) {
		t.Errorf("the message bar says %q", a.lib.notice.Text)
	}
	f.wantUserFolderKept(t, "opening an earlier film")
	f.writeAll(t, item)
	f.wantUserFolderKept(t, "writing an earlier film")
	// Its terms are the ones it had and those added, as its folder holds them; the copy is taken once.
	if terms, _ := media.LoadTerms(filepath.Join(home, ".video.terms.json")); len(terms) != 3 {
		t.Fatalf("the earlier film's terms are %v", terms)
	}
	a.closeFilm()
	f.open(t, "0123456789abcdef")
	if terms, _ := media.LoadTerms(filepath.Join(home, ".video.terms.json")); len(terms) != 3 {
		t.Fatalf("opened again, the earlier film's terms are %v", terms)
	}
}

// A video added without a job from outside the library is given its folder there as it opens, the files an earlier Kando wrote beside it copied in; its job, once it makes its subtitles, keeps that folder. Cancelled the first time it makes them, the job takes the half it made to the Trash with the folder, and the film is back without subtitles; the user's folder is never written.
func TestAnAddedVideosFilesGoIntoItsFolder(t *testing.T) {
	f := newHomeFixture(t)
	a := f.a
	trashed := fakeTrash(t)
	if a.library.AddVideos(testVideos, time.Now(), f.old, f.fresh) != 2 {
		t.Fatal("the videos were not added")
	}
	a.refreshLibrary()
	refresh(f.tt, a)

	// A phone writes to it before it was ever opened: it is given its folder first.
	home := shelf.HomeFor(testVideos, f.old, shelf.AddedKey(f.old))
	if _, err := a.phoneStateCorrect(testVideos, phoneFilmOf(homeFilm(t, a, f.old), phoneShelf{}), homePhoneEdit(phoneFilmID(homeFilm(t, a, f.old)), "手机改的"), nil, false); err != nil {
		t.Fatal(err)
	}
	if got := a.library.AddedIn(testVideos)[f.old].Home; got != home || !fileExists(filepath.Join(home, ".video.edits.json")) {
		t.Fatalf("written from a phone, the added video's folder is %q", got)
	}
	f.wantUserFolderKept(t, "a phone writing to an added video")
	item := f.open(t, f.old)
	if got := a.library.AddedIn(testVideos)[f.old].Home; got != home || item.Home != home || item.SubtitlePath != filepath.Join(home, "video.bilingual.srt") {
		t.Fatalf("the added video's folder is %q, the film's %q, its subtitles %q", got, item.Home, item.SubtitlePath)
	}
	f.writeAll(t, item)
	f.wantUserFolderKept(t, "writing an added video")
	// The library read again lists it with the subtitles in its folder.
	a.refreshLibrary()
	if again := homeFilm(t, a, f.old); again.SubtitlePath != item.SubtitlePath || again.Home != home {
		t.Fatalf("read again, the film is %+v", again)
	}

	// The video added without subtitles is made; stopped the first time, the half it made goes.
	a.closeFilm()
	fresh := homeFilm(t, a, f.fresh)
	a.prefs.language = "简体中文"
	id, msg, _ := a.queueFromFolderChecked(fresh)
	if msg != "" {
		t.Fatal(msg)
	}
	// One never opened has no folder yet: its job gives it one as it starts, named after the job.
	job, _ := a.library.Find(id)
	freshHome := shelf.HomeFor(testVideos, f.fresh, id)
	if job.Home != freshHome {
		t.Fatalf("the job of an added video has the folder %q, want %q", job.Home, freshHome)
	}
	waitUntil(t, f.tt, a, f.updates, func() bool {
		job, _ := a.library.Find(id)
		return job.Stage.Done() && a.queue.active == ""
	})
	f.wantUserFolderKept(t, "making an added video")
	job, _ = a.library.Find(id)
	if job.Stage != core.StageComplete || filepath.Dir(job.SubtitlePath) != freshHome {
		t.Fatalf("the added video's job ended %s with subtitles %q", job.Stage, job.SubtitlePath)
	}
	if _, still := a.library.AddedIn(testVideos)[f.fresh]; still {
		t.Fatal("the video is still added without a job once its job completed")
	}

	// A job that never completed leaves its folder to the Trash as it leaves the shelf.
	never := core.Job{ID: "fedcba9876543210", Source: core.FileURL(f.fresh), Stage: core.StageFailed, LibraryDir: testVideos, VideoPath: f.fresh, Home: filepath.Join(testVideos, "half [fedcba98]")}
	if err := os.MkdirAll(never.Home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(media.BilingualPath(never.FileBase()), []byte(homeOldBilingual), 0o644); err != nil {
		t.Fatal(err)
	}
	a.library.Jobs = append(a.library.Jobs, never)
	a.remove(never.ID)
	if !slices.Contains(*trashed, never.Home) {
		t.Fatalf("the folder of a job that never completed stayed: trashed %v", *trashed)
	}
	f.wantUserFolderKept(t, "removing a job that never completed")
}

// Moved to another library bundle, a film of a video kept outside takes the folder of its files there, its video staying where it is; it cannot move to a folder of the user's opened as a library, which would keep them beside the video. Removed from the library, the folder goes to the Trash and the video stays.
func TestAnOutsideFilmsFolderMovesAndGoesWithIt(t *testing.T) {
	f := newHomeFixture(t)
	a := f.a
	trashed := fakeTrash(t)
	job := f.queue(t, f.fresh)
	other := filepath.Join(t.TempDir(), "other"+media.LibraryExt)
	plain := t.TempDir()
	for _, dir := range []string{other} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	a.edit(func(s *session) { s.Libraries = append(s.Libraries, testVideos, other, plain) })

	a.MoveFilm(homeFilm(t, a, job.ID), plain)
	waitUntil(t, f.tt, a, f.updates, func() bool { return a.lib.move.run == nil })
	if moved, _ := a.library.Find(job.ID); moved.LibraryDir != testVideos {
		t.Fatalf("the film moved into a folder of the user's: %+v", moved)
	}
	wantText(t, f.tt, testlocale.Messages().LibraryMoveHomeToFolder(films.Name(homeFilm(t, a, job.ID))))

	a.MoveFilm(homeFilm(t, a, job.ID), other)
	waitUntil(t, f.tt, a, f.updates, func() bool { return a.lib.move.run == nil })
	moved, _ := a.library.Find(job.ID)
	if moved.LibraryDir != other || !films.InDir(other, moved.Home) || filepath.Dir(moved.SubtitlePath) != moved.Home || !fileExists(moved.SubtitlePath) || moved.VideoPath != f.fresh {
		t.Fatalf("moved, the job is %+v", moved)
	}
	if _, err := os.Stat(job.Home); err == nil {
		t.Fatal("the folder stayed in the library it moved from")
	}
	f.wantUserFolderKept(t, "moving the film")

	a.Choose(other)
	refresh(f.tt, a)
	a.TrashFilms(homeFilm(t, a, job.ID))
	refresh(f.tt, a)
	if !slices.Equal(*trashed, []string{moved.Home}) {
		t.Fatalf("trashed %v, want the film's folder alone", *trashed)
	}
	f.wantUserFolderKept(t, "removing the film")
}

// The folder of a film's files renamed in Finder is found again by the [id] in its name, the subtitles with it; one not found keeps the film on the shelf.
func TestARenamedHomeIsFoundAgain(t *testing.T) {
	f := newHomeFixture(t)
	a := f.a
	job := f.queue(t, f.fresh)
	// A video the user drops into the folder is not gathered into it as the film's video.<ext> (media.GatherVideos).
	dropped := filepath.Join(job.Home, "dropped.mp4")
	if err := os.WriteFile(dropped, []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.refreshLibrary()
	if _, err := os.Stat(dropped); err != nil {
		t.Fatalf("the video dropped into the film's folder was renamed: %v", err)
	}
	if err := os.Remove(dropped); err != nil {
		t.Fatal(err)
	}
	renamed := filepath.Join(testVideos, "My Lesson ["+job.ID[:8]+"]")
	if err := os.Rename(job.Home, renamed); err != nil {
		t.Fatal(err)
	}
	a.refreshLibrary()
	got, _ := a.library.Find(job.ID)
	if got.Home != renamed || got.SubtitlePath != filepath.Join(renamed, "video.bilingual.srt") {
		t.Fatalf("after the rename the job is %+v", got)
	}
	if err := os.RemoveAll(renamed); err != nil {
		t.Fatal(err)
	}
	a.refreshLibrary()
	if _, ok := a.library.Find(job.ID); !ok {
		t.Fatal("a film whose folder is gone was dropped")
	}
}

// No code outside packages/media and packages/track builds the name of a file Kando keeps of a film from a suffix of its own (media.CorrectionsPath and the others name them): a name built so would not follow the film into its folder in the library (core.FileBase), and would write beside a video the user keeps outside it.
func TestNoFilmFileNameIsBuiltByHand(t *testing.T) {
	root := filepath.Join("..", "..")
	suffix := []string{`".srt"`, `".bilingual.srt"`, `".kando.srt"`, `".edits.json"`, `".terms.json"`, `".chapters.json"`, `".proofs.json"`, `".suggestions.json"`, `".reference.srt"`, `".track."`}
	var found []string
	for _, dir := range []string{filepath.Join(root, "apps"), filepath.Join(root, "packages")} {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				if rel == "packages/media" || rel == "packages/track" || d.Name() == "node_modules" || d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "catalog_gen.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(data), "\n") {
				code, _, _ := strings.Cut(line, "//")
				if !strings.Contains(code, "+") && !strings.Contains(code, "Join(") {
					continue
				}
				for _, s := range suffix {
					if strings.Contains(code, s) {
						found = append(found, rel+":"+strconv.Itoa(i+1)+": "+strings.TrimSpace(line))
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(found) > 0 {
		t.Fatalf("film file names built by hand, which a film kept outside the library would write beside its video; use packages/media's:\n%s", strings.Join(found, "\n"))
	}
}

func ptr(s string) *string { return &s }

// homeFilm is the open library's film of key, a job's ID or a video's path, or the job's film of another library.
func homeFilm(t *testing.T, a *app, key string) films.Film {
	t.Helper()
	if item, ok := films.Find(a.films(), key); ok {
		return item
	}
	return mustFilm(t, a, key)
}

// homePhoneEdit is a paired phone's correction of the first line of the film whose ID in the protocol is film, its translation made text.
func homePhoneEdit(film, text string) []psync.Record {
	return phonesync.CorrectionRecords(film, []core.Correction{{Span: core.Span{Start: 2 * time.Second, End: 3 * time.Second}, Translation: &text}})
}

// A film given its folder by a newer Kando, then saved by one that knew no Home, keeps its subtitles in that folder and has the folder back as the newer Kando opens it again, copying nothing again; the library's folder renamed takes the films' folders with it.
func TestAHomeComesBackAndFollowsTheLibrary(t *testing.T) {
	f := newHomeFixture(t)
	a := f.a
	home := filepath.Join(testVideos, "old talk [01234567]")
	if err := os.Mkdir(home, 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(home, "video.bilingual.srt")
	if err := os.WriteFile(sub, []byte(homeOldBilingual), 0o644); err != nil {
		t.Fatal(err)
	}
	a.library.Jobs = append(a.library.Jobs, core.Job{ID: "0123456789abcdef", Source: core.FileURL(f.old), Stage: core.StageComplete, CompletedAt: time.Now(), VideoPath: f.old, SubtitlePath: sub, LibraryDir: testVideos})
	refresh(f.tt, a)
	f.open(t, "0123456789abcdef")
	job, _ := a.library.Find("0123456789abcdef")
	if job.Home != home || job.SubtitlePath != sub {
		t.Fatalf("the job is %+v", job)
	}
	if entries, _ := os.ReadDir(home); len(entries) != 1 {
		t.Fatalf("the folder holds %d files, want its subtitles alone", len(entries))
	}
	f.wantUserFolderKept(t, "opening the film again")

	// An export saved over the subtitles an earlier Kando left beside the video is the user's file, not the film's own, and is not taken back.
	kept := subexportKeep(job.FileBase(), job.SubtitlePath)
	if ours, _, _ := subexportUndo(kept, job.FileBase(), media.BilingualPath(f.old), false); ours {
		t.Fatal("an export over the old subtitles beside the video was taken for the film's own")
	}

	a.closeFilm()
	renamed := filepath.Join(filepath.Dir(testVideos), "renamed"+media.LibraryExt)
	a.retargetLibraryPrefix(testVideos, renamed)
	job, _ = a.library.Find("0123456789abcdef")
	if job.Home != filepath.Join(renamed, "old talk [01234567]") || job.SubtitlePath != filepath.Join(job.Home, "video.bilingual.srt") {
		t.Fatalf("after the library's folder was renamed the job is %+v", job)
	}
}

// A job of a video kept outside a library bundle whose folder is not there as the job starts, as one on a disk taken out while the job waited, fails rather than writing the film's files beside the video; so does one of a library that is locked.
func TestAnOutsideFilmOfALibraryGoneIsNotMade(t *testing.T) {
	f := newHomeFixture(t)
	a := f.a
	gone := filepath.Join(t.TempDir(), "away"+media.LibraryExt)
	if err := os.Mkdir(gone, 0o755); err != nil {
		t.Fatal(err)
	}
	a.edit(func(s *session) { s.Libraries = append(s.Libraries, gone) })
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	job, err := a.newJob(core.FileURL(f.fresh))
	if err != nil {
		t.Fatal(err)
	}
	job.LibraryDir = gone
	a.queueAdd(job)
	waitUntil(t, f.tt, a, f.updates, func() bool {
		got, _ := a.library.Find(job.ID)
		return got.Stage.Done() && a.queue.active == ""
	})
	if got, _ := a.library.Find(job.ID); got.Stage == core.StageComplete || got.Home != "" {
		t.Fatalf("the job of a library that is gone ended %s with the folder %q", got.Stage, got.Home)
	}
	f.wantUserFolderKept(t, "the job of a library that is gone")
}

// A video whose name is too long for a folder's, with the [id] its folder takes, still has one in the library, and is made.
func TestAnOutsideFilmOfALongNameHasAFolder(t *testing.T) {
	f := newHomeFixture(t)
	long := filepath.Join(f.user, strings.Repeat("a", 250)+".mp4")
	if err := os.WriteFile(long, []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.before = folderHashes(t, f.user)
	job := f.queue(t, long)
	if job.Home == "" || filepath.Dir(job.SubtitlePath) != job.Home {
		t.Fatalf("the film of a long name has the folder %q, its subtitles %q", job.Home, job.SubtitlePath)
	}
	f.wantUserFolderKept(t, "making the film of a long name")
}

// failingLocalPipeline is localPipeline whose speech recognition fails, as a film's first making does that is broken or cancelled partway through; with a gate, only once the test closes it.
type failingLocalPipeline struct {
	localPipeline
	gate chan struct{}
}

func (p *failingLocalPipeline) Hear(ctx context.Context, _ string, _, _ time.Duration, _ func(core.Segment)) (string, error) {
	if p.gate != nil {
		select {
		case <-p.gate:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "", errors.New("the speech recognition broke")
}

// An added video opened, given terms and then made for the first time, whose making fails, keeps the folder it was given as it opened, with the terms in it, and the film open on it goes on writing there; so does one whose job made a folder of its own and failed while the film was open on it, which is given its folder again as the film goes back to the added video.
func TestAnOpenFilmKeepsWritingIntoTheLibraryAsItsJobFails(t *testing.T) {
	f := newHomeFixture(t)
	a := f.a
	trashed := fakeTrash(t)
	second := filepath.Join(f.user, "lesson two.mp4")
	if err := os.WriteFile(second, []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.before = folderHashes(t, f.user)
	if a.library.AddVideos(testVideos, time.Now(), f.fresh, second) != 2 {
		t.Fatal("the videos were not added")
	}
	a.refreshLibrary()
	refresh(f.tt, a)
	a.prefs.language = "简体中文"
	openAdded := func(video string) {
		t.Helper()
		a.Open(homeFilm(t, a, video))
		waitUntil(t, f.tt, a, f.updates, func() bool { return a.player.subs.ready && a.gloss.filmRead })
	}
	wantInLibrary := func(after string) {
		t.Helper()
		if !films.InDir(testVideos, a.player.subs.base) || !films.InDir(testVideos, a.player.film.infoBase) || !films.InDir(testVideos, a.glossaryBase()) {
			t.Fatalf("%s, the film open names its files after %q, %q and %q", after, a.player.subs.base, a.player.film.infoBase, a.glossaryBase())
		}
		if !a.glossaryPut(core.Term{Source: "after", Target: "之后", Scope: core.ScopeFilm, Origin: core.OriginUser}) {
			t.Fatal("the term was not written")
		}
		a.SetFilmChapterUse(core.ChaptersEven)
		f.wantUserFolderKept(t, after)
	}

	// Opened first, it has its folder before its job does, and the job shares it.
	openAdded(f.fresh)
	home := a.library.AddedIn(testVideos)[f.fresh].Home
	if home == "" || !a.glossaryPut(core.Term{Source: "before", Target: "之前", Scope: core.ScopeFilm, Origin: core.OriginUser}) {
		t.Fatalf("the added video opened has the folder %q", home)
	}
	a.pipeline = &failingLocalPipeline{}
	a.startFilm(homeFilm(t, a, f.fresh))
	waitUntil(t, f.tt, a, f.updates, func() bool { return a.queue.active == "" && len(a.library.Jobs) == 0 })
	a.refreshLibrary()
	refresh(f.tt, a)
	if got := a.library.AddedIn(testVideos)[f.fresh].Home; got != home || slices.Contains(*trashed, home) {
		t.Fatalf("after its making failed, the added video's folder is %q, trashed %v", got, *trashed)
	}
	if terms, _ := media.LoadTerms(media.TermsPath(core.FileBase(f.fresh, home))); len(terms) != 1 {
		t.Fatalf("the terms written before the making are %v", terms)
	}
	waitUntil(t, f.tt, a, f.updates, func() bool { return a.gloss.filmRead })
	wantInLibrary("writing to an added video whose making failed")
	a.closeFilm()

	// Never opened, its job makes a folder of its own; the film opened on the job goes back to the added video as the job fails.
	gate := make(chan struct{})
	a.pipeline = &failingLocalPipeline{gate: gate}
	id, msg, _ := a.queueFromFolderChecked(homeFilm(t, a, second))
	if msg != "" {
		t.Fatal(msg)
	}
	waitUntil(t, f.tt, a, f.updates, func() bool { return a.queue.active == id })
	a.Open(homeFilm(t, a, id))
	refresh(f.tt, a)
	close(gate)
	waitUntil(t, f.tt, a, f.updates, func() bool { return a.queue.active == "" && len(a.library.Jobs) == 0 })
	a.refreshLibrary()
	refresh(f.tt, a)
	if item, ok := a.openFilm(); !ok || item.Key != second || item.Home == "" {
		t.Fatalf("after its job failed the film open is %+v", item)
	}
	waitUntil(t, f.tt, a, f.updates, func() bool { return a.player.subs.ready && a.gloss.filmRead })
	wantInLibrary("writing to an added video whose job failed while it was open")
}

// A film that opened without being given its folder, as one beside whose video a file cannot be read, and that a paired phone then gives it, reads its files there from then on: what is corrected on the desktop is written there, and kept as it opens again.
func TestAnOpenFilmFollowsTheFolderItIsGiven(t *testing.T) {
	f := newHomeFixture(t)
	a := f.a
	reference := media.ReferencePath(f.old)
	if err := os.WriteFile(reference, []byte(homeOldLine), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(reference); err == nil {
		t.Skip("a file of no permissions is read here")
	}
	const id = "0123456789abcdef"
	a.library.Jobs = append(a.library.Jobs, core.Job{ID: id, Source: core.FileURL(f.old), Language: "简体中文", Stage: core.StageComplete, CompletedAt: time.Now(), VideoPath: f.old, SubtitlePath: media.BilingualPath(f.old), LibraryDir: testVideos})
	refresh(f.tt, a)
	item := f.open(t, id)
	if item.Home != "" {
		t.Fatalf("the film was given the folder %q", item.Home)
	}
	if err := os.Chmod(reference, 0o644); err != nil {
		t.Fatal(err)
	}
	f.before = folderHashes(t, f.user)
	phone := phoneFilmOf(item, phoneShelf{})
	if _, err := a.phoneStateCorrect(testVideos, phone, homePhoneEdit(phone.ID, "手机改的"), nil, false); err != nil {
		t.Fatal(err)
	}
	job, _ := a.library.Find(id)
	if job.Home == "" {
		t.Fatal("the phone did not give the film its folder")
	}
	waitUntil(t, f.tt, a, f.updates, func() bool { return a.correctLoaded() && a.player.subs.path == job.SubtitlePath })
	if a.player.subs.base != job.FileBase() || a.player.film.infoBase != job.FileBase() || a.glossaryBase() != job.FileBase() {
		t.Fatalf("the film open names its files after %q, %q and %q, not %q", a.player.subs.base, a.player.film.infoBase, a.glossaryBase(), job.FileBase())
	}
	// What the desktop writes is the film's corrections as it read them, the phone's among them, and its own.
	if !a.correctWrite(append(slices.Clone(a.player.subs.edits), core.Correction{Span: core.Span{Start: 4 * time.Second, End: 5 * time.Second}, Translation: ptr("桌面改的")})) {
		t.Fatal("the correction was not written")
	}
	f.wantUserFolderKept(t, "correcting the film once a phone gave it its folder")
	a.closeFilm()
	f.open(t, id)
	var kept []string
	for _, edit := range a.player.subs.edits {
		if edit.Translation != nil {
			kept = append(kept, *edit.Translation)
		}
	}
	if !slices.Contains(kept, "桌面改的") || !slices.Contains(kept, "手机改的") {
		t.Fatalf("opened again, the film's corrections are %q", kept)
	}
}

// A film's folder renamed in Finder before the library was read again is followed as the film is next written to, not made again by its old name.
func TestARenamedHomeIsFollowedBeforeTheLibraryIsReadAgain(t *testing.T) {
	f := newHomeFixture(t)
	a := f.a
	job := f.queue(t, f.fresh)
	renamed := filepath.Join(testVideos, "My Lesson ["+job.ID[:8]+"]")
	if err := os.Rename(job.Home, renamed); err != nil {
		t.Fatal(err)
	}
	item := f.open(t, job.ID)
	if _, err := os.Stat(job.Home); err == nil {
		t.Fatal("the folder was made again by its old name")
	}
	got, _ := a.library.Find(job.ID)
	if got.Home != renamed || item.Home != renamed || got.SubtitlePath != filepath.Join(renamed, "video.bilingual.srt") || a.player.subs.path != got.SubtitlePath {
		t.Fatalf("opened after the rename, the job is %+v, the film's subtitles %q", got, a.player.subs.path)
	}
}
