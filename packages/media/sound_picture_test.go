package media

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// skipWithoutFFmpeg skips a test that needs ffmpeg when ffmpeg is missing or cannot make its clip. CI sets KANDO_REQUIRE_FFMPEG where it installs ffmpeg, and there the test fails instead, so that it cannot pass by skipping.
func skipWithoutFFmpeg(t *testing.T, format string, args ...any) {
	t.Helper()
	if os.Getenv("KANDO_REQUIRE_FFMPEG") != "" {
		t.Fatalf(format, args...)
	}
	t.Skipf(format, args...)
}

// testFFmpeg is ffmpeg on PATH, or skips the test.
func testFFmpeg(t *testing.T) string {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		skipWithoutFFmpeg(t, "ffmpeg is not installed")
	}
	return ffmpeg
}

// writeSound writes seconds of a tone that comes and goes, as speech does, to path, with cover art when cover is set.
func writeSound(t *testing.T, ffmpeg, path string, seconds int, cover bool) {
	t.Helper()
	ctx := context.Background()
	tone := "sine=f=440:d=" + strconv.Itoa(seconds) + ",volume='if(lt(mod(t,1),0.5),1,0.1)':eval=frame"
	args := []string{"-nostdin", "-loglevel", "error", "-y", "-f", "lavfi", "-i", tone}
	if cover {
		art := filepath.Join(t.TempDir(), "art.png")
		if _, err := run(ctx, ffmpeg, "-nostdin", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "color=c=red:s=300x300:d=1", "-frames:v", "1", art); err != nil {
			skipWithoutFFmpeg(t, "ffmpeg could not make cover art: %v", err)
		}
		args = append(args, "-i", art, "-map", "0", "-map", "1", "-c:v", "png", "-disposition:v", "attached_pic", "-id3v2_version", "3")
	}
	if _, err := run(ctx, ffmpeg, append(args, path)...); err != nil {
		skipWithoutFFmpeg(t, "ffmpeg could not make a sound: %v", err)
	}
}

// A file of sound alone, with or without cover art, has a picture drawn from its sound: 16:9, wide, bright enough to keep.
func TestGrabPosterOfSound(t *testing.T) {
	ffmpeg := testFFmpeg(t)
	for _, cover := range []bool{false, true} {
		dir := t.TempDir()
		sound := filepath.Join(dir, "lesson.mp3")
		writeSound(t, ffmpeg, sound, 40, cover)
		dest := filepath.Join(dir, "poster.jpg")
		if err := GrabPoster(context.Background(), ffmpeg, sound, dest); err != nil {
			t.Fatalf("cover %v: %v", cover, err)
		}
		data, err := os.ReadFile(dest)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width != soundPosterWidth || cfg.Height != soundPosterWidth*9/16 {
			t.Fatalf("cover %v: poster %+v %v, want %dx%d", cover, cfg, err, soundPosterWidth, soundPosterWidth*9/16)
		}
		if PosterTooDark(dest) {
			t.Fatalf("cover %v: a sound's poster counts as a black frame, so it would be grabbed again", cover)
		}
		// The clip cards of a sound film show its sound from where each starts.
		frame, err := GrabFrame(context.Background(), ffmpeg, sound, 10*time.Second)
		if err != nil {
			t.Fatalf("cover %v: frame: %v", cover, err)
		}
		if cfg, err := jpeg.DecodeConfig(bytes.NewReader(frame)); err != nil || cfg.Width != posterWidth {
			t.Fatalf("cover %v: frame %+v %v", cover, cfg, err)
		}
	}
}

// The header tells a film's length and whether it has a picture; cover art is not one.
func TestReadHeader(t *testing.T) {
	ffmpeg := testFFmpeg(t)
	dir := t.TempDir()
	bare, art := filepath.Join(dir, "bare.mp3"), filepath.Join(dir, "art.mp3")
	writeSound(t, ffmpeg, bare, 12, false)
	writeSound(t, ffmpeg, art, 12, true)
	video := filepath.Join(dir, "clip.mp4")
	if _, err := run(context.Background(), ffmpeg, "-nostdin", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "color=c=blue:s=160x90:d=3", "-c:v", "mpeg4", video); err != nil {
		skipWithoutFFmpeg(t, "ffmpeg could not make a clip: %v", err)
	}
	for _, tc := range []struct {
		path    string
		picture bool
		length  time.Duration
	}{{bare, false, 12 * time.Second}, {art, false, 12 * time.Second}, {video, true, 3 * time.Second}} {
		h := ReadHeader(ffmpeg, tc.path)
		if !h.Read || h.Picture != tc.picture || h.Length < tc.length-time.Second || h.Length > tc.length+time.Second {
			t.Errorf("%s: %+v, want picture %v and %s", filepath.Base(tc.path), h, tc.picture, tc.length)
		}
		if Duration(ffmpeg, tc.path) != h.Length {
			t.Errorf("%s: Duration differs from the header", filepath.Base(tc.path))
		}
	}
	if h := ReadHeader(ffmpeg, filepath.Join(dir, "gone.mp3")); h.Read {
		t.Errorf("a missing file was read: %+v", h)
	}
}

// Square art becomes 16:9 with all of it showing; a picture near 16:9 stays as it is.
func TestWideCover(t *testing.T) {
	ffmpeg := testFFmpeg(t)
	square := encodeJPEG(t, 300, 300)
	wide, err := WideCover(context.Background(), ffmpeg, square)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(wide))
	if err != nil || cfg.Width != soundPosterWidth || cfg.Height != soundPosterWidth*9/16 {
		t.Fatalf("wide cover %+v %v", cfg, err)
	}
	near := encodeJPEG(t, 320, 180)
	if got, err := WideCover(context.Background(), ffmpeg, near); err != nil || !bytes.Equal(got, near) {
		t.Fatalf("a 16:9 picture was redrawn: %v", err)
	}
}

func encodeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
