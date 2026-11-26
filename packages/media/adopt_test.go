package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kando/packages/core"
	"kando/packages/track"
)

// GatherVideos takes every file Kando names after a loose video into its folder with it, renamed after video.<ext>: its document's logs, its terms, its chapters and the other hidden files beside it, not only its subtitles and corrections, so the film keeps its lines, its glossary and its cut in its folder.
func TestGatherVideosTakesEveryFileOfTheVideo(t *testing.T) {
	dir := t.TempDir()
	names := []string{"talk.mp4", "talk.bilingual.srt", ".talk.edits.json", ".talk.terms.json", ".talk.chapters.json", ".talk.proofs.json", ".talk.suggestions.json", ".talk.reference.srt", ".talk.track.a1.jsonl", ".talk.track.b2.jsonl", ".other.terms.json"}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := GatherVideos(dir, nil); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(dir, "talk")
	for _, name := range names[:len(names)-1] {
		next := filepath.Join(folder, strings.Replace(name, "talk", "video", 1))
		if data, err := os.ReadFile(next); err != nil || string(data) != name {
			t.Errorf("%s is not %s in the film's folder: %v", name, filepath.Base(next), err)
		}
	}
	if logs := track.Logs(filepath.Join(folder, "video.mp4")); len(logs) != 2 {
		t.Fatalf("the film's logs in its folder are %v", logs)
	}
	if _, err := os.Stat(filepath.Join(dir, ".other.terms.json")); err != nil {
		t.Fatal("a file of another video's name went with the film")
	}
}

// AdoptFiles copies what an earlier Kando wrote beside a video into the folder its film's files are named after from now on, under that name: the transcript kept aside from the user's own subtitles becomes the film's transcript there, and the user's subtitles, the video and the files of other videos stay out. Nothing beside the video changes, a file already in the folder is kept as it is, and the device's log is told the subtitles it took in under their old name are the same under their new one, so they are not taken in again.
func TestAdoptFilesCopiesKandosFilesAlone(t *testing.T) {
	user, home := t.TempDir(), filepath.Join(t.TempDir(), "lib.kando", "talk [12345678]")
	if err := os.Mkdir(filepath.Dir(home), 0o755); err != nil {
		t.Fatal(err)
	}
	video := filepath.Join(user, "talk.mp4")
	files := map[string]string{
		"talk.mp4":           "v",
		"talk.srt":           "1\n00:00:00,000 --> 00:00:01,000\nthe user's\n\n",
		"talk.kando.srt":     "1\n00:00:00,000 --> 00:00:01,000\nhello\n\n",
		"talk.bilingual.srt": "1\n00:00:00,000 --> 00:00:01,000\n你好\nhello\n\n",
		".talk.edits.json":   "{}",
		"other.srt":          "x",
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(user, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The document took in the bilingual subtitles as they are.
	doc, _ := track.Read(video)
	events, err := trackFiles(doc, BilingualPath(video), TranscriptOf(video))
	if err != nil || len(events) == 0 {
		t.Fatalf("taking in the subtitles: %v %v", events, err)
	}
	if _, err := track.Append(video, "dev", events...); err != nil {
		t.Fatal(err)
	}
	beside := map[string][]byte{}
	entries, _ := os.ReadDir(user)
	for _, e := range entries {
		beside[e.Name()], _ = os.ReadFile(filepath.Join(user, e.Name()))
	}

	base := core.FileBase(video, home)
	placed, err := AdoptFiles(video, base, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if placed[BilingualPath(video)] != filepath.Join(home, "video.bilingual.srt") || placed[asideTranscriptPath(video)] != filepath.Join(home, "video.srt") || placed[CorrectionsPath(video)] != filepath.Join(home, ".video.edits.json") {
		t.Fatalf("placed %v", placed)
	}
	if _, ok := placed[transcriptPath(video)]; ok {
		t.Fatal("the user's own subtitles were copied as the film's")
	}
	if data, _ := os.ReadFile(filepath.Join(home, "video.srt")); string(data) != files["talk.kando.srt"] {
		t.Fatalf("the film's transcript holds %q", data)
	}
	for _, name := range []string{"video.mp4", "other.srt", "video.kando.srt"} {
		if _, err := os.Stat(filepath.Join(home, name)); err == nil {
			t.Errorf("%s is in the film's folder", name)
		}
	}
	entries, _ = os.ReadDir(user)
	if len(entries) != len(beside) {
		t.Fatalf("the user's folder holds %d files, had %d", len(entries), len(beside))
	}
	for _, e := range entries {
		if data, _ := os.ReadFile(filepath.Join(user, e.Name())); string(data) != string(beside[e.Name()]) {
			t.Errorf("%s beside the video changed", e.Name())
		}
	}
	// Read in the folder, the subtitles are what the document holds: nothing to take in.
	after, _ := track.Read(base)
	if again, err := trackFiles(after, BilingualPath(base), TranscriptOf(base)); err != nil || len(again) != 0 {
		t.Fatalf("the subtitles in the folder are taken in again: %v %v", again, err)
	}

	// Copied again, as after a crash before the film was given its folder, what is there stays.
	if err := os.WriteFile(filepath.Join(home, ".video.edits.json"), []byte(`{"kept":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := AdoptFiles(video, base, "dev"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(home, ".video.edits.json")); string(data) != `{"kept":true}` {
		t.Fatalf("a file in the folder was written over: %q", data)
	}

	// A library folder that is not there is not made again.
	gone := filepath.Join(t.TempDir(), "gone.kando", "talk [12345678]")
	if _, err := AdoptFiles(video, core.FileBase(video, gone), "dev"); err == nil {
		t.Fatal("copied into a library that is not there")
	}
	if _, err := os.Stat(filepath.Dir(gone)); err == nil {
		t.Fatal("the library's folder was made")
	}
}

// AdoptFiles copies the logs of several devices, a megabyte each, quickly enough to run as a film opens.
func TestAdoptFilesCopiesLogsQuickly(t *testing.T) {
	user, root := t.TempDir(), t.TempDir()
	video := filepath.Join(user, "talk.mp4")
	if err := os.WriteFile(video, []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	line := strings.Repeat("x", 1023) + "\n"
	for _, device := range []string{"a1", "b2", "c3", "d4", "e5"} {
		if err := os.WriteFile(track.LogPath(video, device), []byte(strings.Repeat(line, 1024)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	placed, err := AdoptFiles(video, core.FileBase(video, filepath.Join(root, "talk [12345678]")), "")
	took := time.Since(start)
	if err != nil || len(placed) != 5 {
		t.Fatalf("placed %v: %v", placed, err)
	}
	t.Logf("copying 5 logs of 1 MB took %v", took)
	if took > 2*time.Second {
		t.Fatalf("copying 5 logs of 1 MB took %v", took)
	}
}
