package media

import (
	"os"
	"path/filepath"
	"testing"

	"kando/packages/core"
)

// A video of a folder of the user's opened as a library keeps its film's files in the hidden folder beside it; one a bundle holds, inside the folder or as the library, and one kept outside the folder, have none there.
func TestFolderHomeFor(t *testing.T) {
	root := filepath.Join("/", "Movies")
	video := filepath.Join(root, "Show", "ep 1.mkv")
	if got := FolderHomeFor(root, video); got != filepath.Join(root, "Show", FilesDir, "ep 1") {
		t.Fatal(got)
	}
	for _, c := range []struct{ root, video string }{
		{filepath.Join("/", "lib"+LibraryExt), filepath.Join("/", "lib"+LibraryExt, "Talk", "video.mp4")},
		{root, filepath.Join(root, "Mine"+LibraryExt, "Talk", "video.mp4")},
		{root, filepath.Join("/", "Elsewhere", "talk.mp4")},
		{root, root},
	} {
		if got := FolderHomeFor(c.root, c.video); got != "" {
			t.Errorf("%s of %s has the folder %q", c.video, c.root, got)
		}
	}
	for name, want := range map[string]string{"..": "_", ".hidden": "_.hidden", "a: b?": "a_ b_", "ends. ": "ends"} {
		if got := FolderName(name); got != want {
			t.Errorf("FolderName(%q) = %q, want %q", name, got, want)
		}
	}
}

// A film is given its folder beside its video once, with what an earlier Kando wrote beside it copied in and that left as it was: given again, as by another computer sharing the folder, it is kept as it is, so what was taken out of it does not come back. A copy that fails leaves no folder, so a later try copies again.
func TestAdoptIntoGivesAFolderOnce(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "talk.mp4")
	for path, data := range map[string]string{video: "v", BilingualPath(video): "1\n00:00:00,000 --> 00:00:01,000\n你好\nhello\n\n", CorrectionsPath(video): "[]"} {
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	home := FolderHome(video)
	if err := os.Chmod(CorrectionsPath(video), 0); err == nil {
		if _, err := os.ReadFile(CorrectionsPath(video)); err != nil {
			if _, _, err := AdoptInto(video, home, "dev"); err == nil {
				t.Fatal("a file that cannot be read was copied")
			}
			if _, err := os.Stat(home); err == nil {
				t.Fatal("a copy that failed left the film's folder")
			}
		}
		os.Chmod(CorrectionsPath(video), 0o644)
	}
	placed, made, err := AdoptInto(video, home, "dev")
	base := core.FileBase(video, home)
	if err != nil || !made || placed[BilingualPath(video)] != BilingualPath(base) || !fileExists(CorrectionsPath(base)) {
		t.Fatalf("placed %v, made %v: %v", placed, made, err)
	}
	if !fileExists(BilingualPath(video)) {
		t.Fatal("the file beside the video went")
	}
	if FolderVideoChanged(home, video) {
		t.Fatal("the video it was given for is taken for another")
	}
	if err := os.Remove(CorrectionsPath(base)); err != nil {
		t.Fatal(err)
	}
	if _, made, err := AdoptInto(video, home, "dev"); err != nil || made || fileExists(CorrectionsPath(base)) {
		t.Fatalf("given again: made %v, %v; the corrections taken out came back: %v", made, err, fileExists(CorrectionsPath(base)))
	}
	// Another file in its place is told apart by its size; kept, it is the folder's from then on.
	if err := os.WriteFile(video, []byte("another"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !FolderVideoChanged(home, video) {
		t.Fatal("another file in the video's place is not told apart")
	}
	if err := KeepFolderVideo(home, video); err != nil || FolderVideoChanged(home, video) {
		t.Fatalf("kept, it is still told apart: %v", err)
	}
	// Listed, the film shows the subtitles in its folder.
	videos, err := ListVideos(dir)
	if err != nil || len(videos) != 1 || videos[0].Home != home || videos[0].Subtitles != BilingualPath(base) {
		t.Fatalf("listed %+v: %v", videos, err)
	}
	// A library bundle it is added to takes its files from its folder, not from beside it.
	bundle := filepath.Join(t.TempDir(), "talk [12345678]")
	if err := os.WriteFile(BilingualPath(base), []byte("1\n00:00:00,000 --> 00:00:01,000\n新的\nhello\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := AdoptFiles(video, core.FileBase(video, bundle), ""); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(BilingualPath(core.FileBase(video, bundle))); string(data) != "1\n00:00:00,000 --> 00:00:01,000\n新的\nhello\n\n" {
		t.Fatalf("the bundle took %q", data)
	}
}
