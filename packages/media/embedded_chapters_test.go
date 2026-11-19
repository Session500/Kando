package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"kando/packages/core"
)

// ffmpegChapters is what ffmpeg -i writes of an audiobook with three chapters, the last without a title, and a title of its own above them.
const ffmpegChapters = `Input #0, mov,mp4,m4a,3gp,3g2,mj2, from 'book.m4b':
  Metadata:
    title           : Book
  Duration: 00:00:12.00, start: 0.000000, bitrate: 71 kb/s
  Chapters:
    Chapter #0:0: start 0.000000, end 4.000000
      Metadata:
        title           : 第一章 はじめに
    Chapter #0:1: start 4.000000, end 8.000000
      Metadata:
        title           : Chapter: Two, the end
    Chapter #0:2: start 8.500000, end 12.000000
      Metadata:
        title           : 
  Stream #0:0[0x1](und): Audio: aac (LC) (mp4a / 0x6134706D), 44100 Hz, mono, fltp, 69 kb/s (default)
    Metadata:
      title           : SoundHandler
`

// Each chapter a file marks is read with its start and title, one without a title named by its number; the file's own title and a stream's are not chapters' titles.
func TestParseChapters(t *testing.T) {
	want := []core.Chapter{chapter(0, "第一章 はじめに", ""), chapter(4, "Chapter: Two, the end", ""), chapter(8.5, "3", "")}
	if got := parseChapters(ffmpegChapters); !reflect.DeepEqual(got, want) {
		t.Fatalf("chapters %+v, want %+v", got, want)
	}
	if got := parseChapters("Input #0, mp3, from 'x.mp3':\n  Duration: 00:01:00.00\n  Stream #0:0: Audio: mp3\n"); got != nil {
		t.Fatalf("a file without chapters has %+v", got)
	}
}

// An audiobook's chapters are read from its file and kept beside it, so the cut the user picks is kept with them; chapters kept already are what it has.
func TestKeepEmbeddedChapters(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		skipWithoutFFmpeg(t, "ffmpeg is not installed")
	}
	dir := t.TempDir()
	meta := filepath.Join(dir, "meta.txt")
	if err := os.WriteFile(meta, []byte(";FFMETADATA1\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=4000\ntitle=One\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=4000\nEND=8000\ntitle=Two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	book := filepath.Join(dir, "book.m4b")
	if _, err := run(context.Background(), ffmpeg, "-nostdin", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "sine=d=8", "-i", meta, "-map", "0", "-map_chapters", "1", "-c:a", "aac", "-f", "mp4", book); err != nil {
		skipWithoutFFmpeg(t, "ffmpeg could not make an audiobook: %v", err)
	}
	got, ok := KeepEmbeddedChapters(ffmpeg, book)
	want := []core.Chapter{chapter(0, "One", ""), chapter(4, "Two", "")}
	if !ok || !reflect.DeepEqual(got.Chapters, want) {
		t.Fatalf("chapters %+v %v", got, ok)
	}
	if kept, ok := ReadChapters(book); !ok || !reflect.DeepEqual(kept.Chapters, want) {
		t.Fatalf("kept %+v %v", kept, ok)
	}
	if err := SetChapterUse(book, core.ChaptersEven); err != nil {
		t.Fatal(err)
	}
	if again, _ := KeepEmbeddedChapters(ffmpeg, book); again.Use != core.ChaptersEven {
		t.Fatalf("the cut picked was lost: %+v", again)
	}
	plain := filepath.Join(dir, "plain.m4a")
	if _, err := run(context.Background(), ffmpeg, "-nostdin", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "sine=d=2", "-c:a", "aac", plain); err != nil {
		skipWithoutFFmpeg(t, "ffmpeg could not make a sound: %v", err)
	}
	if _, ok := KeepEmbeddedChapters(ffmpeg, plain); ok {
		t.Fatal("a file without chapters has some")
	}
	if _, err := os.Stat(ChaptersPath(plain)); !os.IsNotExist(err) {
		t.Fatal("a file without chapters had a chapters file written")
	}
}

// chapteredFilm is makeFilm reading the chapters a local file marks in itself.
type chapteredFilm struct {
	*makeFilm
	marks []core.Chapter
}

func (f chapteredFilm) EmbeddedChapters(context.Context, string) []core.Chapter { return f.marks }

// A local film keeps the chapters its file marks, as a link keeps its site's.
func TestMakeKeepsTheChaptersALocalFileMarks(t *testing.T) {
	job, video := makeJob(t)
	marks := []core.Chapter{chapter(0, "Opening", ""), chapter(30, "Story", "")}
	done, _ := makeFilmOf(t, chapteredFilm{&makeFilm{length: 60 * time.Second, language: "ja"}, marks}, job, 0)
	if done.Stage != core.StageComplete {
		t.Fatalf("job ended %s: %v", done.Stage, done.Err)
	}
	kept, ok := ReadChapters(video)
	if !ok || len(kept.Chapters) != 2 || kept.Chapters[1].Title != "Story" {
		t.Fatalf("kept %+v %v", kept, ok)
	}
}
