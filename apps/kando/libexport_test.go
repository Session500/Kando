package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kando/apps/kando/libexport"
	"kando/packages/media"
)

// libexportRunAll runs the open library's export as the dialog does and waits for its end.
func libexportRunAll(t *testing.T, l *userFolderLibrary) libexport.Result {
	t.Helper()
	a := l.a
	a.libexport.open = false
	a.OpenLibexport()
	if !a.LibexportOpen() {
		t.Fatal("the dialog did not open")
	}
	a.LibexportStart()
	waitUntil(t, l.tt, a, l.updates, func() bool { return a.LibexportDone() })
	result := a.LibexportResult()
	a.CloseLibexport()
	return result
}

// A folder of the user's opened as a library exports each film's subtitles beside its video, as the player shows them, named in the language slot players read; made again it leaves what it wrote as it is, writes its own over once the film changed, and never a file the user changed. What an earlier Kando left beside a video is offered to go to the Trash, but for the transcript under the video's own name.
func TestALibraryExportsBesideItsVideos(t *testing.T) {
	l := newUserFolderLibrary(t)
	a := l.a
	trashed := fakeTrash(t)
	l.queue(t, l.fresh)
	l.open(t, l.old)
	a.closeFilm()
	a.refreshLibrary()

	result := libexportRunAll(t, l)
	if result.Written != 2 || result.Failed != 0 {
		t.Fatalf("the export did %+v", result)
	}
	fresh := filepath.Join(filepath.Dir(l.fresh), "lesson one.zh.srt")
	old := filepath.Join(filepath.Dir(l.old), "old talk.dual.srt")
	for _, path := range []string{fresh, old} {
		if !fileExists(path) {
			t.Fatalf("%s was not exported", filepath.Base(path))
		}
	}
	// The earlier film's export holds the correction the user made of its line.
	if data, _ := os.ReadFile(old); !strings.Contains(string(data), "您好") {
		t.Fatalf("the export holds %q", data)
	}
	if !fileExists(filepath.Join(filepath.Dir(l.fresh), media.FilesDir, libexport.LedgerName)) {
		t.Fatal("no ledger of what was exported")
	}

	// Again, nothing is written; a file the user changed is kept as it is.
	if err := os.WriteFile(fresh, []byte("the user's own\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result = libexportRunAll(t, l)
	if result.Written != 0 || result.Same != 1 || result.Kept != 1 {
		t.Fatalf("exported again, the export did %+v", result)
	}
	if data, _ := os.ReadFile(fresh); string(data) != "the user's own\n" {
		t.Fatalf("the user's file holds %q", data)
	}

	// The earlier film's files beside its video are offered to go, and go to the Trash.
	if a.LibexportOld() != 1 {
		t.Fatalf("%d films' old files are offered", a.LibexportOld())
	}
	a.LibexportTrashOld()
	refresh(l.tt, a)
	if !slices.Contains(*trashed, media.BilingualPath(l.old)) || !slices.Contains(*trashed, media.CorrectionsPath(l.old)) {
		t.Fatalf("trashed %v", *trashed)
	}
	for _, path := range *trashed {
		if filepath.Dir(path) != filepath.Dir(l.old) || !strings.HasPrefix(filepath.Base(path), ".old talk.") && !strings.HasPrefix(filepath.Base(path), "old talk.") || strings.HasSuffix(path, "old talk.srt") {
			t.Fatalf("%s went to the Trash", path)
		}
	}
}

// Into a folder, the export is laid out as the library is; a library bundle's own films can only go into one.
func TestALibraryExportsIntoAFolder(t *testing.T) {
	l := newUserFolderLibrary(t)
	a := l.a
	l.queue(t, l.fresh)
	a.refreshLibrary()
	out := t.TempDir()
	a.libexport.folder = out
	a.OpenLibexport()
	a.SetLibexportBeside(false)
	a.LibexportStart()
	waitUntil(t, l.tt, a, l.updates, func() bool { return a.LibexportDone() })
	if got := a.LibexportResult(); got.Written != 2 {
		t.Fatalf("the export did %+v", got)
	}
	if !fileExists(filepath.Join(out, "Show", "lesson one.zh.srt")) || !fileExists(filepath.Join(out, "Show", "old talk.dual.srt")) || !fileExists(filepath.Join(out, libexport.LedgerName)) {
		t.Fatal("the export is not laid out as the library is")
	}
	a.CloseLibexport()
	l.wantFolderKept(t, "exporting into a folder")

	// A bundle's own film is not exported beside its video, which is Kando's.
	f := newHomeFixture(t)
	f.a.prefs.language = "简体中文"
	inside := filepath.Join(testVideos, "Talk", "talk.mp4")
	if err := os.MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inside, []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.queue(t, inside)
	f.a.refreshLibrary()
	if f.a.LibexportFilms() != 1 || f.a.LibexportBesideOK() {
		t.Fatalf("a bundle's film: %d films, beside %v", f.a.LibexportFilms(), f.a.LibexportBesideOK())
	}
}
