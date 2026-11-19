package sync

import "testing"

// A phone names what it downloads by its type, videos and sounds alike, and a desktop serves each file with the type that names it back.
func TestMediaTypesNameFilesBothWays(t *testing.T) {
	for kind, want := range map[string]string{
		"video/mp4": ".mp4", "video/quicktime": ".mov", "video/x-matroska": ".mkv", "video/webm; codecs=vp9": ".webm",
		"audio/mpeg": ".mp3", "audio/mp4": ".m4a", "audio/ogg; codecs=opus": ".ogg", "audio/flac": ".flac", "image/png": ".png",
		"application/octet-stream": ".mp4", "": ".mp4",
	} {
		if got := Extension(kind, ".mp4"); got != want {
			t.Errorf("Extension(%q) = %q, want %q", kind, got, want)
		}
	}
	for _, name := range []string{"video.mp4", "video.MKV", "video.mp3", "video.m4a", "video.wav", "video.flac", "poster.jpg"} {
		kind := MediaType(name)
		if got := Extension(kind, ""); got == "" {
			t.Errorf("%s is served as %q, which the phone does not name", name, kind)
		}
	}
	if got := MediaType("book.m4b"); got != "audio/mp4" {
		t.Errorf("an audiobook is served as %q", got)
	}
	if got := MediaType("notes"); got != "application/octet-stream" {
		t.Errorf("a file without an extension is served as %q", got)
	}
}

func TestAsksWith(t *testing.T) {
	if !AsksWith("subtitles_only,sound", WithSound) || !AsksWith("subtitles_only", WithSubtitlesOnly) || AsksWith("subtitles_only", WithSound) || AsksWith("", WithSound) {
		t.Fatal("AsksWith misreads the kinds asked for")
	}
}
