package media

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// listed is the paths ListVideos lists under dir, relative to it, sorted.
func listed(t *testing.T, dir string, away ...string) []string {
	t.Helper()
	videos, err := ListVideos(dir, away...)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, v := range videos {
		rel, _ := filepath.Rel(dir, v.Path)
		out = append(out, rel)
	}
	slices.Sort(out)
	return out
}

// Files of sound alone are films, in the root and in folders, as videos are.
func TestListVideosTakesSound(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"episode 1.mp3":                "x",
		"Track 02.M4A":                 "x",
		"book/video.m4b":               "x",
		"listening/part.opus":          "x",
		"notes.txt":                    "x",
		"cover.jpg":                    "x",
		"listening/part.bilingual.srt": "x",
	})
	want := []string{"Track 02.M4A", "book/video.m4b", "episode 1.mp3", "listening/part.opus"}
	if got := listed(t, dir); !slices.Equal(got, want) {
		t.Fatalf("listed %q, want %q", got, want)
	}
}

// A sound beside a video of the same name is the video's, not a film of its own, also beside a video moved away; two videos of one name make one film, the first by the order of the extensions.
func TestListVideosTakesOneFilmOfAName(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"lesson.mp4":       "x",
		"lesson.mp3":       "x",
		"talk.mkv":         "x",
		"talk.MP4":         "x",
		"course/video.mp4": "x",
		"course/video.m4a": "x",
		"stored/video.mp3": "x",
		"alone.mp3":        "x",
	})
	away := []string{filepath.Join(dir, "stored", "video.mp4")}
	want := []string{"alone.mp3", "course/video.mp4", "lesson.mp4", "talk.MP4"}
	if got := listed(t, dir, away...); !slices.Equal(got, want) {
		t.Fatalf("listed %q, want %q", got, want)
	}
	if !IsFilm(filepath.Join(dir, "alone.mp3")) || IsFilm(filepath.Join(dir, "lesson.mp3")) || !IsFilm(filepath.Join(dir, "lesson.mp4")) {
		t.Fatal("IsFilm does not take one film of a name")
	}
	got := SameNamed(filepath.Join(dir, "lesson.mp4"))
	if !slices.Equal(got, []string{filepath.Join(dir, "lesson.mp3")}) {
		t.Fatalf("SameNamed = %q", got)
	}
	if got := SameNamed(filepath.Join(dir, "lesson.mp3")); len(got) != 0 {
		t.Fatalf("the sound has the video as its own: %q", got)
	}
}

// The clips of Kando's own export for Anki are not films, where they are and when the library gathers loose files, which would break the notes that play them.
func TestListVideosLeavesExportedClips(t *testing.T) {
	dir := t.TempDir()
	clip := ClipName(0x1234abcd5678ef90)
	writeFiles(t, dir, map[string]string{
		clip: "x",
		filepath.Join("Kando Saved Lines audio", clip):        "x",
		filepath.Join("Kando Saved Lines audio", ClipName(1)): "x",
	})
	if got := listed(t, dir); len(got) != 0 {
		t.Fatalf("listed %q", got)
	}
	if moved, err := GatherVideos(dir, nil); err != nil || len(moved) != 0 {
		t.Fatalf("gathered %v %v", moved, err)
	}
	if IsFilm(filepath.Join(dir, clip)) {
		t.Fatal("an exported clip is a film")
	}
}

// A loose video takes the sound of its name into its folder, as video.<ext> beside video.<ext>.
func TestGatherVideosTakesTheSoundOfItsName(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"lesson.mp4": "v", "lesson.mp3": "s", "lesson.srt": "x", "podcast.mp3": "p"})
	moved, err := GatherVideos(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	for src, dst := range map[string]string{
		"lesson.mp4":  "lesson/video.mp4",
		"lesson.mp3":  "lesson/video.mp3",
		"lesson.srt":  "lesson/video.srt",
		"podcast.mp3": "podcast/video.mp3",
	} {
		if moved[filepath.Join(dir, src)] != filepath.Join(dir, dst) {
			t.Fatalf("%s went to %q, want %s (moved %v)", src, moved[filepath.Join(dir, src)], dst, moved)
		}
	}
	if got := listed(t, dir); !slices.Equal(got, []string{"lesson/video.mp4", "podcast/video.mp3"}) {
		t.Fatalf("listed %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "lesson.mp3")); !os.IsNotExist(err) {
		t.Fatal("the lesson's sound stayed in the root")
	}
}
