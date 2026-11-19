package media

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"kando/packages/core"
)

func TestHearHandsOnEachLineInTheFilmsTime(t *testing.T) {
	dir := t.TempDir()
	ffmpegArgs := filepath.Join(dir, "ffmpeg-args")
	ffmpeg := writeFakeTool(t, dir, "ffmpeg", fakeTool{Args: ffmpegArgs})
	whisper := writeFakeTool(t, dir, "whisper-cli", fakeTool{
		Act:        "transcribe",
		Transcript: `{"result":{"language":"ja"},"transcription":[]}`,
		Stdout: "\n" +
			"[00:00:00.000 --> 00:00:03.360]   おはよう\n" +
			"whisper_print_timings: something else\n" +
			"[00:00:05.100 --> 00:00:07.920]   \n" +
			"[00:00:28.000 --> 00:00:31,500]  またね\n",
	})
	tools := Tools{FFmpeg: ffmpeg, Whisper: whisper, WhisperModel: filepath.Join(dir, "model.bin")}
	var heard []core.Segment
	language, err := tools.Hear(context.Background(), filepath.Join(dir, "film.mkv"), time.Minute, 90*time.Second, func(s core.Segment) { heard = append(heard, s) })
	if err != nil {
		t.Fatal(err)
	}
	want := []core.Segment{
		{Start: 60 * time.Second, End: 63360 * time.Millisecond, Text: "おはよう"},
		{Start: 88 * time.Second, End: 91500 * time.Millisecond, Text: "またね"},
	}
	if !slices.Equal(heard, want) || language != "ja" {
		t.Fatalf("heard %+v in %q, want %+v in ja", heard, language, want)
	}
	data, err := os.ReadFile(ffmpegArgs)
	if err != nil {
		t.Fatal(err)
	}
	// Only the part of the sound asked for is taken from the video, seeking before it is read.
	args := strings.Split(strings.TrimSpace(string(data)), "\n")
	if i := slices.Index(args, "-ss"); i < 0 || args[i+1] != "60.000" || slices.Index(args, "-i") < i {
		t.Fatalf("ffmpeg %q, want -ss 60.000 before -i", args)
	}
	if i := slices.Index(args, "-t"); i < 0 || args[i+1] != "30.000" {
		t.Fatalf("ffmpeg %q, want -t 30.000", args)
	}
}

func TestHearToTheEndTakesNoLength(t *testing.T) {
	dir := t.TempDir()
	ffmpegArgs := filepath.Join(dir, "ffmpeg-args")
	tools := Tools{
		FFmpeg:       writeFakeTool(t, dir, "ffmpeg", fakeTool{Args: ffmpegArgs}),
		Whisper:      writeFakeTool(t, dir, "whisper-cli", fakeTool{Act: "transcribe", Transcript: `{"result":{"language":"en"}}`}),
		WhisperModel: filepath.Join(dir, "model.bin"),
	}
	if _, err := tools.Hear(context.Background(), filepath.Join(dir, "film.mkv"), 0, 0, func(core.Segment) {}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(ffmpegArgs)
	if slices.Contains(strings.Split(string(data), "\n"), "-t") {
		t.Fatalf("ffmpeg %q, want no -t", data)
	}
}

func TestParseWhisperLine(t *testing.T) {
	for _, tc := range []struct {
		line string
		want core.Segment
		ok   bool
	}{
		{"[00:00:01.000 --> 00:00:03.360]   Hello.", core.Segment{Start: time.Second, End: 3360 * time.Millisecond, Text: "Hello."}, true},
		{"[101:02:03.004 --> 101:02:04,005] long", core.Segment{Start: 101*time.Hour + 2*time.Minute + 3*time.Second + 4*time.Millisecond, End: 101*time.Hour + 2*time.Minute + 4*time.Second + 5*time.Millisecond, Text: "long"}, true},
		{"[00:00:01.000 --> 00:00:02.000]", core.Segment{}, false},
		{"whisper_init_from_file: loading model", core.Segment{}, false},
		{"", core.Segment{}, false},
	} {
		got, ok := parseWhisperLine(tc.line)
		if got != tc.want || ok != tc.ok {
			t.Errorf("parseWhisperLine(%q) = %+v, %v; want %+v, %v", tc.line, got, ok, tc.want, tc.ok)
		}
	}
}

// A link is made first from its sound, which has no picture: its length is read all the same, and a file whose header does not say, as a webm a browser recorded, is as long as decoding it reaches.
func TestLengthOfSoundAlone(t *testing.T) {
	dir := t.TempDir()
	for name, stderr := range map[string]string{
		"sound":       "Input #0, matroska,webm, from 'video.sound':\n  Duration: 00:01:05.01, start: 0.000000, bitrate: 79 kb/s\n  Stream #0:0: Audio: opus, 48000 Hz, mono, fltp\n",
		"no duration": "  Duration: N/A, start: 0.000000, bitrate: N/A\n  Stream #0:0: Audio: opus, 48000 Hz, mono, fltp\nsize=N/A time=00:00:31.00 bitrate=N/A\rsize=N/A time=00:01:05.00 bitrate=N/A speed=1.4e+03x\n",
	} {
		ffmpeg := writeFakeTool(t, filepath.Join(dir, name), "ffmpeg", fakeTool{Stderr: stderr})
		if got := (Tools{FFmpeg: ffmpeg}).Length(context.Background(), "video.sound"); got/time.Second != 65 {
			t.Errorf("%s: length %v", name, got)
		}
	}
}
