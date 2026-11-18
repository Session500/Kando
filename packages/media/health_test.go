package media

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// TestProbe runs fake programs that answer, fail, cannot load a library or do not answer in time, and checks what Probe makes of each.
func TestProbe(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		fake fakeTool
		want Health
	}{
		{"answers", fakeTool{Stdout: "ffmpeg version 9.0.2 Copyright (c) 2000-2026 the FFmpeg developers\n"}, Health{State: HealthReady, Version: "9.0.2"}},
		{"missing-library", fakeTool{Stderr: "dyld[14576]: Library not loaded: /opt/homebrew/opt/libbluray/lib/libbluray.2.dylib\n  Referenced from: /opt/homebrew/Cellar/ffmpeg/7.1.1_3/bin/ffmpeg\n", Exit: 1}, Health{State: HealthBroken, Problem: ProblemLibrary, Library: "libbluray"}},
		{"fails", fakeTool{Stderr: "something went wrong\n", Exit: 2}, Health{State: HealthBroken, Problem: ProblemFailed}},
		{"no-version", fakeTool{Stdout: "error: unknown argument: --version\n"}, Health{State: HealthReady}},
	}
	for _, c := range cases {
		path := writeFakeTool(t, filepath.Join(dir, c.name), "ffmpeg", c.fake)
		if got := Probe(context.Background(), "ffmpeg", path); got != c.want {
			t.Errorf("%s: Probe = %+v, want %+v", c.name, got, c.want)
		}
	}
	if got := Probe(context.Background(), "ffmpeg", filepath.Join(dir, "gone", "ffmpeg")); got.State != HealthBroken || got.Problem != ProblemCannotStart {
		t.Errorf("a program that is not there: Probe = %+v, want it broken, unable to start", got)
	}
	// A program that has not answered in time is not known to be broken: it may still be going through macOS's first-run check.
	slow := writeFakeTool(t, filepath.Join(dir, "slow"), "ffmpeg", fakeTool{Wait: true})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if got := Probe(ctx, "ffmpeg", slow); got != (Health{}) {
		t.Errorf("a program that did not answer in time: Probe = %+v, want HealthUnknown", got)
	}
}

// TestBrokenHealth checks how the ways a program fails to start read, on each system.
func TestBrokenHealth(t *testing.T) {
	cases := []struct {
		code int
		out  string
		want Health
	}{
		{-1, "dyld[7208]: Library not loaded: @rpath/libavdevice.62.dylib\n", Health{State: HealthBroken, Problem: ProblemLibrary, Library: "libavdevice"}},
		{127, "whisper-cli: error while loading shared libraries: libggml.so.0: cannot open shared object file: No such file or directory\n", Health{State: HealthBroken, Problem: ProblemLibrary, Library: "libggml"}},
		{dllNotFound, "", Health{State: HealthBroken, Problem: ProblemLibrary}},
		{dllNotFoundSigned, "", Health{State: HealthBroken, Problem: ProblemLibrary}},
		// macOS stops a program whose signature it refuses before it prints anything.
		{-1, "", Health{State: HealthBroken, Problem: ProblemCrashed}},
		{1, "Traceback (most recent call last):\n", Health{State: HealthBroken, Problem: ProblemFailed}},
	}
	for _, c := range cases {
		if got := brokenHealth(c.code, c.out); got != c.want {
			t.Errorf("brokenHealth(%d, %q) = %+v, want %+v", c.code, c.out, got, c.want)
		}
	}
}

func TestVersionIn(t *testing.T) {
	cases := map[string]string{
		"ffmpeg version 9.0.2 Copyright (c) 2000-2026 the FFmpeg developers\nbuilt with Apple clang": "9.0.2",
		"2026.10.26\n":                         "2026.10.26",
		"whisper-cli version 1.9.5\n":          "1.9.5",
		"error: unknown argument: --version\n": "",
		"usage: whisper-cli [options] file0\n": "",
		"":                                     "",
	}
	for out, want := range cases {
		if got := versionIn(out); got != want {
			t.Errorf("versionIn(%q) = %q, want %q", out, got, want)
		}
	}
}
