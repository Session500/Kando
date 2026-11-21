package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"kando/packages/core"
)

// Hearer is a pipeline that hears part of a video at a time, handing each line on as it is heard (Tools.Hear), so a film's lines come while it plays rather than once the whole of it is heard.
type Hearer interface {
	Hear(ctx context.Context, videoPath string, from, to time.Duration, heard func(core.Segment)) (string, error)
}

// Hear recognizes the speech of the video at videoPath from from to to, handing heard each line, in the film's time, as whisper-cli prints it, and returns the language it heard, as its code, "" when it does not say. Only that part of the sound is taken from the video, so hearing starts at once wherever it is in the film; to of 0, or past the end, hears to the end. The language heard is the one the context names for the film (withSpeechLanguage), else the one Tools names, else the one whisper finds. Where whisper-cli can find speech, only the speech is heard and the language is told from it (hearSpeech); else the whole of the sound is.
func (t Tools) Hear(ctx context.Context, videoPath string, from, to time.Duration, heard func(core.Segment)) (string, error) {
	if strings.TrimSpace(videoPath) == "" {
		return "", errors.New("video path is empty")
	}
	dir, err := os.MkdirTemp("", "kando-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)

	// whisper.cpp expects 16 kHz mono 16-bit PCM; -ss before -i seeks without decoding what comes before.
	audio := filepath.Join(dir, "audio.wav")
	args := []string{"-nostdin", "-loglevel", "error", "-y", "-ss", ffmpegTime(from), "-i", videoPath}
	if to > from {
		args = append(args, "-t", ffmpegTime(to-from))
	}
	args = append(args, "-vn", "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", audio)
	if _, err := runStage(ctx, t.FFmpeg, args...); err != nil {
		if NoAudioStream(err.Error()) {
			return "", ErrNoAudio
		}
		return "", err
	}
	if t.hearsSpeech(ctx) {
		language, err := t.hearSpeech(ctx, dir, audio, videoPath, from, to, heard)
		if !errors.Is(err, errNoSpeechSearch) {
			return language, err
		}
	}
	_, language := t.hearing(ctx)
	if language == "" {
		language = "auto"
	}
	out := filepath.Join(dir, "transcript")
	// whisper-cli prints each line on stdout as it decodes it, 30 seconds of sound at a time, even with -np.
	follow := func(line string) {
		if s, ok := parseWhisperLine(line); ok {
			s.Start += from
			s.End += from
			heard(s)
		}
	}
	if _, _, err := runFollowing(ctx, false, follow, t.Whisper, t.WhisperArgs(language, out, audio)...); err != nil {
		return "", err
	}
	data, err := os.ReadFile(out + ".json")
	if err != nil {
		return "", err
	}
	_, spoken, err := parseWhisperJSON(data)
	return spoken, err
}

// ffmpegTime is d as ffmpeg takes a time: seconds, to the millisecond.
func ffmpegTime(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', 3, 64)
}

// whisperLine is a line whisper-cli prints as it hears it: "[00:00:01.000 --> 00:00:03.360]   text".
var whisperLine = regexp.MustCompile(`^\[(\d+):(\d\d):(\d\d)[.,](\d{3}) --> (\d+):(\d\d):(\d\d)[.,](\d{3})\]\s*(.*)$`)

// parseWhisperLine reads a line whisper-cli prints as it hears it, in the time of the sound it was given; false for any other line, and for one that holds no words.
func parseWhisperLine(line string) (core.Segment, bool) {
	m := whisperLine.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return core.Segment{}, false
	}
	text := strings.TrimSpace(m[9])
	if text == "" {
		return core.Segment{}, false
	}
	at := func(h, mi, s, ms string) time.Duration {
		n := func(v string) time.Duration { x, _ := strconv.Atoi(v); return time.Duration(x) }
		return n(h)*time.Hour + n(mi)*time.Minute + n(s)*time.Second + n(ms)*time.Millisecond
	}
	return core.Segment{Start: at(m[1], m[2], m[3], m[4]), End: at(m[5], m[6], m[7], m[8]), Text: text}, true
}
