package media

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Health is whether a program Kando runs starts and answers, as Probe found by running it once to ask its version. A program can be on disk and still not start: a Homebrew ffmpeg whose libraries Homebrew upgraded under it stops before it reads its arguments.
type Health struct {
	State HealthState
	// Version is the version the program said it is, "" when it said none Kando reads.
	Version string
	// Problem is why a Broken program did not run. Library is the library it could not load, for ProblemLibrary when its message names one, without the folder or the version ("libbluray" for /opt/homebrew/opt/libbluray/lib/libbluray.2.dylib).
	Problem Problem
	Library string
}

// HealthState is what running the program once showed.
type HealthState int

const (
	// HealthUnknown is a program not asked yet, or one that did not answer within ProbeTimeout: macOS checks a program the first time it runs, which can take a while for a large one such as yt-dlp, so a slow answer says nothing about whether it works.
	HealthUnknown HealthState = iota
	// HealthReady is a program that started and exited cleanly.
	HealthReady
	// HealthBroken is a program that did not start, or exited with an error or was stopped by the system before it answered.
	HealthBroken
)

// Problem is why a program is Broken, which the app words for the user.
type Problem string

const (
	// ProblemLibrary is a library the program links that the system could not load: missing, or of another version than it was built with.
	ProblemLibrary Problem = "library"
	// ProblemCrashed is a program the system stopped, as macOS does one whose library is missing or whose signature it refuses.
	ProblemCrashed Problem = "crashed"
	// ProblemFailed is a program that exited with an error.
	ProblemFailed Problem = "failed"
	// ProblemCannotStart is a program the system would not start at all: not executable, built for another processor, or gone.
	ProblemCannotStart Problem = "cannot-start"
)

// ProbeTimeout is how long Probe waits for a program's answer. The first run of a large program on macOS waits for its scan, so it is as long as YTDLPVersion waits.
const ProbeTimeout = 30 * time.Second

// Probe runs the program at path once to ask its version, as program names it ("yt-dlp", "ffmpeg", "whisper-cli"), and reports whether it started and answered. It waits at most ProbeTimeout, and less when ctx ends first; a program that has not answered by then is HealthUnknown.
func Probe(ctx context.Context, program, path string) Health {
	ctx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()
	var out []byte
	err := retryBusy(ctx, func() error {
		var err error
		out, err = Command(ctx, path, versionArgs(program)...).CombinedOutput()
		return err
	})
	if ctx.Err() != nil {
		return Health{}
	}
	var exit *exec.ExitError
	switch {
	case err == nil:
		return Health{State: HealthReady, Version: versionIn(string(out))}
	case errors.As(err, &exit):
		return brokenHealth(exit.ExitCode(), string(out))
	default:
		return Health{State: HealthBroken, Problem: ProblemCannotStart}
	}
}

// versionArgs asks program its version: ffmpeg takes -version, the others --version.
func versionArgs(program string) []string {
	if program == "ffmpeg" {
		return []string{"-version"}
	}
	return []string{"--version"}
}

// The ways a system says a program could not load a library it links: macOS's dyld ("Library not loaded: @rpath/libx.2.dylib") and Linux's loader ("error while loading shared libraries: libx.so.164: cannot open shared object file").
var (
	dyldMissing   = regexp.MustCompile(`Library not loaded: (\S+)`)
	loaderMissing = regexp.MustCompile(`error while loading shared libraries: ([^\s:]+)`)
)

// Windows's exit status for a program one of whose DLLs it could not find (STATUS_DLL_NOT_FOUND), as ExitCode reads it on Windows and as a signed 32-bit number elsewhere.
const (
	dllNotFound       = 0xC0000135
	dllNotFoundSigned = -1073741515
)

// brokenHealth is the health of a program that exited with code, -1 for one the system stopped with a signal, after printing out.
func brokenHealth(code int, out string) Health {
	h := Health{State: HealthBroken, Problem: ProblemFailed}
	if m := dyldMissing.FindStringSubmatch(out); m != nil {
		h.Problem, h.Library = ProblemLibrary, libraryName(m[1])
	} else if m := loaderMissing.FindStringSubmatch(out); m != nil {
		h.Problem, h.Library = ProblemLibrary, libraryName(m[1])
	} else if code == dllNotFound || code == dllNotFoundSigned {
		h.Problem = ProblemLibrary
	} else if code == -1 {
		h.Problem = ProblemCrashed
	}
	return h
}

// libraryName is the name of the library file at path without its folder, version or extension: libbluray for libbluray.2.dylib, libx264 for libx264.so.164.
func libraryName(path string) string {
	name := filepath.Base(path)
	if i := strings.IndexByte(name, '.'); i > 0 {
		name = name[:i]
	}
	return name
}

// versionIn is the version a program printed for its version flag, "" when the first line it printed is no version, as an older whisper-cli that does not know --version complains instead.
func versionIn(out string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	line = strings.TrimSpace(line)
	lower := strings.ToLower(line)
	if line == "" || strings.Contains(lower, "error") || strings.Contains(lower, "unknown") || strings.Contains(lower, "usage") {
		return ""
	}
	return shortVersion(line)
}

// shortVersion is the version in the first line a program printed: the third word of ffmpeg's "ffmpeg version 9.0.2 Copyright …", the word after "version" in others', or the line itself, cut to 48 bytes, as yt-dlp prints just its version.
func shortVersion(line string) string {
	fields := strings.Fields(line)
	if len(fields) >= 3 && strings.EqualFold(fields[0], "ffmpeg") && strings.EqualFold(fields[1], "version") {
		return fields[2]
	}
	lower := strings.ToLower(line)
	if i := strings.Index(lower, "version"); i >= 0 {
		rest := strings.Trim(line[i+len("version"):], " :\t")
		if parts := strings.Fields(rest); len(parts) > 0 {
			return strings.Trim(parts[0], ":")
		}
	}
	if len(line) > 48 {
		return line[:48]
	}
	return line
}
