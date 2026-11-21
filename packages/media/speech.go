package media

import (
	"context"
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"kando/packages/core"
)

// Hearing only the speech: whisper, given music, silence or the sounds of a scene, makes up lines ("ご視聴ありがとうございました", "Thank you.") and, as it reads each window after the text before it, says them again window after window; asked which language a film is in, it may answer English for its opening music. So where whisper-cli can find the speech in a sound (its --vad, with the Silero model Kando carries), Hear has it find the speech first and hears only that, a chunk at a time, and tells the language from speech alone.

//go:embed silero/ggml-silero-v5.1.2.bin
var sileroModel []byte

// sileroFile is the name sileroModel is written under, beside the speech model.
const sileroFile = "ggml-silero-v5.1.2.bin"

const (
	// speechThreshold is how sure the VAD must be that a moment is speech, and speechPadMs how far it widens each stretch of speech on either side: below its default of 0.5 and 30 ms, as the faint start of a line is easily left out.
	speechThreshold = "0.35"
	speechPadMs     = "200"
	// speechMaxSeconds splits a stretch of speech longer than a chunk may be.
	speechMaxSeconds = "25"
	// chunkGap is the longest pause within a chunk: stretches of speech closer than it are heard together, as a line heard alone, without the words around it, is more often misheard. chunkMax bounds a chunk below whisper's 30-second window, and chunkPad is the sound kept on either side of its speech.
	chunkGap = 2 * time.Second
	chunkMax = 25 * time.Second
	chunkPad = 300 * time.Millisecond
	// firstStretch is the sound at the start of a run whose speech is found, and heard, before the rest's: the pad before the run and its first cell, which is where the playhead is, so its lines come without waiting for the whole run to be searched.
	firstStretch = 45 * time.Second
	// languageWindow is how much speech the language is told from at a time, repeated to fill the 30 seconds whisper tells it from, and languageWindows how many such windows, spread over the speech, tell it together: whisper pads less sound with silence, and longer stretches of speech found by the VAD, run together, lead it astray as well (over a Japanese film's speech, windows of 4 seconds repeated were told as Japanese 24 times in 24, of 30 seconds 11 times; a few seconds padded with silence were told as Korean five times in six). languageSpeech is the speech below which Hear looks past the run for more.
	languageWindow  = 4 * time.Second
	languageWindows = 7
	languageSpeech  = 20 * time.Second
	// languageAhead is how far past a run Hear looks for speech to tell the language from when the run has too little, as one whose cell is a film's opening music.
	languageAhead = 10 * time.Minute
)

// errNoSpeechSearch is why hearSpeech did not hear a run: whisper-cli could not say where its speech is, and Hear hears the whole of it instead.
var errNoSpeechSearch = errors.New("whisper-cli did not say where the speech is")

// span is a stretch of a sound, from its start.
type span struct{ from, to time.Duration }

// chunk is a stretch of a sound heard as one file, and the stretches of speech in it.
type chunk struct {
	span
	speech []span
}

// speechSupport is, by whisper-cli path, whether that program finds speech (hearsSpeech).
var speechSupport sync.Map

// hearsSpeech reports whether Hear hears only the speech of a video: whisper-cli has --vad and there is a speech model to run it with.
func (t Tools) hearsSpeech(ctx context.Context) bool {
	if t.Whisper == "" || t.WhisperModel == "" {
		return false
	}
	if ok, known := speechSupport.Load(t.Whisper); known {
		return ok.(bool)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, _ := Command(ctx, t.Whisper, "-h").CombinedOutput()
	if ctx.Err() != nil {
		return false
	}
	ok := strings.Contains(string(out), "--vad-model")
	speechSupport.Store(t.Whisper, ok)
	return ok
}

// sileroPath writes the VAD model Kando carries beside the speech model, or in the temporary folder when that folder cannot be written, unless it is there already, and returns where it is.
func (t Tools) sileroPath() (string, error) {
	var err error
	for _, dir := range []string{filepath.Dir(t.WhisperModel), os.TempDir()} {
		path := filepath.Join(dir, sileroFile)
		if info, statErr := os.Stat(path); statErr == nil && info.Size() == int64(len(sileroModel)) {
			return path, nil
		}
		if err = writeFileAtomic(path, sileroModel); err == nil {
			return path, nil
		}
	}
	return "", err
}

// hearSpeech hears the speech of a run whose sound is the WAV at audio, from from in the film: it finds the speech, the first stretch's before the rest's, and hears it a chunk at a time, handing heard each line in the film's time. It returns the language the speech is in: the one ctx or the settings name, else the one told from the speech, "" when the run and the sound after it hold no speech. errNoSpeechSearch says it heard nothing, as whisper-cli could not find the speech, and the run is heard whole.
func (t Tools) hearSpeech(ctx context.Context, dir, audio, videoPath string, from, to time.Duration, heard func(core.Segment)) (string, error) {
	pcm, err := readPCM(audio)
	if err != nil {
		return "", fmt.Errorf("%w: %v", errNoSpeechSearch, err)
	}
	silero, err := t.sileroPath()
	if err != nil {
		return "", fmt.Errorf("%w: %v", errNoSpeechSearch, err)
	}
	length := pcmLength(pcm)
	_, language := t.hearing(ctx)
	if language == "auto" {
		language = ""
	}
	// A run whose language is known hears its first stretch while the rest's speech is found; one whose language is not finds all its speech first, to tell the language from.
	stretches := []span{{0, length}}
	if language != "" && length > firstStretch+10*time.Second {
		stretches = []span{{0, firstStretch + 10*time.Second}, {0, length}}
	}
	var done time.Duration
	for i, stretch := range stretches {
		stretch.from = done
		spans, err := t.findSpeech(ctx, dir, silero, pcm, stretch)
		if err != nil {
			if i == 0 {
				return "", err
			}
			return language, err
		}
		if i == 0 && len(stretches) > 1 {
			// The first stretch ends in the pause nearest its end, so a line is not cut in two between it and the rest.
			done = pauseNear(spans, firstStretch, stretch.to)
			spans = before(spans, done)
		} else {
			done = stretch.to
		}
		if language == "" {
			if language, err = t.tellLanguage(ctx, dir, silero, videoPath, pcm, spans, to); err != nil {
				return "", err
			}
		}
		// Each stretch's chunks stay within it, so speech either side of the cut is not heard twice.
		if err := t.hearChunks(ctx, dir, i, pcm, packChunks(spans, span{stretch.from, done}), from, language, heard); err != nil {
			return language, err
		}
	}
	return language, nil
}

// findSpeech has whisper-cli find the speech of stretch of the sound pcm, and returns where it is in the sound. -dl ends the run once the speech is found, with the language whisper tells from it, which hearSpeech does not use, as whisper pads less than 30 seconds of speech with silence to tell it.
func (t Tools) findSpeech(ctx context.Context, dir, silero string, pcm []byte, stretch span) ([]span, error) {
	path := filepath.Join(dir, "speech.wav")
	if err := writePCM(path, pcmSlice(pcm, stretch)); err != nil {
		return nil, err
	}
	var spans []span
	searched := false
	follow := func(line string) {
		if m := vadSegmentLine.FindStringSubmatch(line); m != nil {
			spans = append(spans, span{stretch.from + vadSeconds(m[1]), stretch.from + vadSeconds(m[2])})
		} else if strings.Contains(line, "speech segments after filtering") {
			searched = true
		}
	}
	args := append(t.whisperBase("auto"), "--vad", "-vm", silero, "-vt", speechThreshold, "-vp", speechPadMs, "-vmsd", speechMaxSeconds, "-dl", "-f", path)
	if _, _, err := runFollowing(ctx, followStderr, follow, t.Whisper, args...); err != nil {
		if ctx.Err() != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", errNoSpeechSearch, err)
	}
	if !searched {
		return nil, errNoSpeechSearch
	}
	return spans, nil
}

// vadSegmentLine is a stretch of speech whisper-cli's VAD prints, in seconds: "whisper_vad_segments_from_probs: VAD segment 0: start = 101.82, end = 104.10 (duration: 2.28)".
var vadSegmentLine = regexp.MustCompile(`VAD segment \d+: start = ([\d.]+), end = ([\d.]+)`)

// vadSeconds is a time the VAD prints, in seconds.
func vadSeconds(s string) time.Duration {
	f, _ := strconv.ParseFloat(s, 64)
	return seconds(f)
}

// pauseNear is where a stretch searched up to end is cut, about at: the middle of the pause between two stretches of speech nearest it, else, with no pause before end, end.
func pauseNear(spans []span, at, end time.Duration) time.Duration {
	cut, best := end, end
	for i := 0; i+1 < len(spans); i++ {
		mid := (spans[i].to + spans[i+1].from) / 2
		if d := (mid - at).Abs(); d < best && spans[i+1].from > spans[i].to {
			cut, best = mid, d
		}
	}
	if len(spans) > 0 && spans[len(spans)-1].to < end {
		// Silence after the last speech: the stretch ends there unless a pause nearer at was found.
		mid := (spans[len(spans)-1].to + end) / 2
		if (mid - at).Abs() < best {
			cut = mid
		}
	}
	return cut
}

// before is spans up to cut, the one across it cut there.
func before(spans []span, cut time.Duration) []span {
	var out []span
	for _, s := range spans {
		if s.from >= cut {
			break
		}
		s.to = min(s.to, cut)
		out = append(out, s)
	}
	return out
}

// packChunks gathers spans of speech into the chunks they are heard in: those less than chunkGap apart, up to chunkMax together, with chunkPad of the sound on each side, within bounds and no further than halfway to the next chunk.
func packChunks(spans []span, bounds span) []chunk {
	var out []chunk
	for _, s := range spans {
		if n := len(out); n > 0 && s.from-out[n-1].to < chunkGap && s.to-out[n-1].from <= chunkMax {
			out[n-1].to = max(out[n-1].to, s.to)
			out[n-1].speech = append(out[n-1].speech, s)
			continue
		}
		out = append(out, chunk{span: s, speech: []span{s}})
	}
	for i := range out {
		lo, hi := bounds.from, bounds.to
		if i > 0 {
			lo = (out[i-1].speech[len(out[i-1].speech)-1].to + out[i].from) / 2
		}
		if i+1 < len(out) {
			hi = (out[i].to + out[i+1].speech[0].from) / 2
		}
		out[i].from = max(lo, out[i].from-chunkPad)
		out[i].to = min(hi, out[i].to+chunkPad)
	}
	return out
}

// tellLanguage tells the language of the speech at spans of the sound pcm, of a run that ends at to in the video at videoPath, and of the speech after it when the run has too little: the language most of languageWindows windows spread over it are told as. "" when there is no speech to tell it from.
func (t Tools) tellLanguage(ctx context.Context, dir, silero, videoPath string, pcm []byte, spans []span, to time.Duration) (string, error) {
	speech := speechOf(pcm, spans)
	if pcmLength(speech) < languageSpeech && to > 0 {
		if ahead, err := t.speechAhead(ctx, dir, silero, videoPath, to); err == nil {
			speech = append(speech, ahead...)
		} else if ctx.Err() != nil {
			return "", err
		}
	}
	length := pcmLength(speech)
	if length == 0 {
		return "", nil
	}
	n := max(1, min(languageWindows, int(length/languageWindow)))
	args := append(t.whisperBase("auto"), "-dl")
	for i := range n {
		at := length * time.Duration(i) / time.Duration(n)
		path := filepath.Join(dir, fmt.Sprintf("language-%d.wav", i))
		if err := writePCM(path, fill(pcmSlice(speech, span{at, at + languageWindow}), 30*time.Second)); err != nil {
			return "", err
		}
		args = append(args, "-f", path)
	}
	var told []string
	follow := func(line string) {
		if m := detectedLine.FindStringSubmatch(line); m != nil {
			told = append(told, m[1])
		}
	}
	if _, _, err := runFollowing(ctx, followStderr, follow, t.Whisper, args...); err != nil {
		return "", err
	}
	return mostOf(told), nil
}

// speechAhead is the speech of the languageAhead of the video at videoPath after to, run together.
func (t Tools) speechAhead(ctx context.Context, dir, silero, videoPath string, to time.Duration) ([]byte, error) {
	audio := filepath.Join(dir, "ahead.wav")
	args := []string{"-nostdin", "-loglevel", "error", "-y", "-ss", ffmpegTime(to), "-i", videoPath, "-t", ffmpegTime(languageAhead), "-vn", "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", audio}
	if _, err := runStage(ctx, t.FFmpeg, args...); err != nil {
		return nil, err
	}
	pcm, err := readPCM(audio)
	if err != nil {
		return nil, err
	}
	spans, err := t.findSpeech(ctx, dir, silero, pcm, span{0, pcmLength(pcm)})
	if err != nil {
		return nil, err
	}
	return speechOf(pcm, spans), nil
}

// detectedLine is the language whisper tells a sound as: "whisper_full_with_state: auto-detected language: ja (p = 0.992840)".
var detectedLine = regexp.MustCompile(`auto-detected language: (\w+)`)

// mostOf is the language told most often, the first told of those told as often.
func mostOf(told []string) string {
	best, count := "", 0
	for _, a := range told {
		n := 0
		for _, b := range told {
			if a == b {
				n++
			}
		}
		if n > count {
			best, count = a, n
		}
	}
	return best
}

// hearChunks hears chunks of the sound pcm, of a run from from in the film, in language, with one whisper-cli run that is given each as a file of its own: whisper reads each file with no text before it, so a line made up in one is not carried into the next. stage names the chunks' files apart from another stage's. whisper-cli prints a file's lines on stdout and, once it has heard the file, that it saves what it heard; the lines are handed on then, in the film's time.
func (t Tools) hearChunks(ctx context.Context, dir string, stage int, pcm []byte, chunks []chunk, from time.Duration, language string, heard func(core.Segment)) error {
	if len(chunks) == 0 {
		return nil
	}
	byPath := map[string]chunk{}
	args := append(t.whisperBase(cmpOr(language, "auto")), "-np", "-oj")
	for i, c := range chunks {
		path := filepath.Join(dir, fmt.Sprintf("chunk-%d-%04d.wav", stage, i))
		if err := writePCM(path, pcmSlice(pcm, c.span)); err != nil {
			return err
		}
		byPath[path] = c
		args = append(args, "-f", path)
	}
	var lines []core.Segment
	follow := func(line string) {
		if s, ok := parseWhisperLine(line); ok {
			lines = append(lines, s)
			return
		}
		if m := savedLine.FindStringSubmatch(line); m != nil {
			if c, ok := byPath[strings.TrimSuffix(m[1], ".json")]; ok {
				for _, s := range collapseRepeats(lines) {
					s = placeLine(s, c)
					s.Start += from
					s.End += from
					heard(s)
				}
			}
			lines = nil
		} else if strings.HasPrefix(line, "error: failed to read audio file") {
			lines = nil
		}
	}
	_, _, err := runFollowing(ctx, followBoth, follow, t.Whisper, args...)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == whisperSomeFilesFailed && ctx.Err() == nil {
		// A chunk that could not be read is left out; the others were heard.
		return nil
	}
	return err
}

// collapseRepeats is lines with each run of the same line said again and again made one line, from the first's start to the last's end: within a chunk whisper may still say a line over a few times, some of them for a split second, where it was said once.
func collapseRepeats(lines []core.Segment) []core.Segment {
	var out []core.Segment
	for _, s := range lines {
		if n := len(out); n > 0 && out[n-1].Text == s.Text {
			out[n-1].End = max(out[n-1].End, s.End)
			continue
		}
		out = append(out, s)
	}
	return out
}

// whisperSomeFilesFailed is whisper-cli's exit status when some of the files it was given could not be read, after it heard the rest.
const whisperSomeFilesFailed = 11

// savedLine is what whisper-cli prints once it heard a file, naming where it saves what it heard: "output_json: saving output to '/tmp/kando-1/chunk-0-0003.wav.json'".
var savedLine = regexp.MustCompile(`saving output to '(.*)'$`)

// placeLine is line s, heard in the time of chunk c's file, in the time of the sound: within c, and starting where its speech does when whisper starts it in a pause before it, as it may start the first line after a pause at the pause.
func placeLine(s core.Segment, c chunk) core.Segment {
	s.Start = min(c.from+s.Start, c.to)
	s.End = min(c.from+s.End, c.to)
	for _, sp := range c.speech {
		if s.Start >= sp.from && s.Start < sp.to {
			break
		}
		if sp.from > s.Start {
			if sp.from < s.End {
				s.Start = sp.from
			}
			break
		}
	}
	return s
}

// whisperBase is the arguments every run of whisper-cli takes: the speech model, the threads, language, and the hints typed in the speech settings.
func (t Tools) whisperBase(language string) []string {
	threads := t.WhisperThreads
	if threads <= 0 {
		threads = WhisperThreadsAuto()
	}
	args := []string{"-m", t.WhisperModel, "-t", strconv.Itoa(threads), "-l", language}
	if hints := WhisperPrompt(t.WhisperHints); hints != "" {
		args = append(args, "--prompt", hints)
	}
	return args
}

// The sound Hear hears is 16 kHz mono 16-bit PCM, as whisper.cpp takes it.
const pcmBytesPerSecond = 16000 * 2

func pcmLength(pcm []byte) time.Duration {
	return time.Duration(len(pcm)) * time.Second / pcmBytesPerSecond
}

// pcmAt is the offset of the sample at d into a sound.
func pcmAt(pcm []byte, d time.Duration) int {
	i := int(d.Seconds()*16000) * 2
	return max(0, min(i, len(pcm)))
}

func pcmSlice(pcm []byte, s span) []byte {
	return pcm[pcmAt(pcm, s.from):pcmAt(pcm, s.to)]
}

// speechOf is the speech at spans of pcm, run together.
func speechOf(pcm []byte, spans []span) []byte {
	var out []byte
	for _, s := range spans {
		out = append(out, pcmSlice(pcm, s)...)
	}
	return out
}

// fill is pcm repeated to last d, or as it is when it is that long already, or empty.
func fill(pcm []byte, d time.Duration) []byte {
	n := int(d.Seconds()*16000) * 2
	if len(pcm) == 0 || len(pcm) >= n {
		return pcm
	}
	out := make([]byte, 0, n)
	for len(out) < n {
		out = append(out, pcm[:min(len(pcm), n-len(out))]...)
	}
	return out
}

// readPCM is the samples of the WAV file at path, which ffmpeg wrote as 16 kHz mono 16-bit PCM.
func readPCM(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, errors.New("not a WAV file")
	}
	for i := 12; i+8 <= len(data); {
		size := int(binary.LittleEndian.Uint32(data[i+4 : i+8]))
		body := i + 8
		if string(data[i:i+4]) == "data" {
			return data[body:min(len(data), body+size)], nil
		}
		i = body + size + size%2
	}
	return nil, errors.New("WAV file has no samples")
}

// writePCM writes pcm to path as a WAV file of 16 kHz mono 16-bit PCM.
func writePCM(path string, pcm []byte) error {
	header := make([]byte, 44)
	copy(header[0:], "RIFF")
	binary.LittleEndian.PutUint32(header[4:], uint32(36+len(pcm)))
	copy(header[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], 1)
	binary.LittleEndian.PutUint16(header[22:], 1)
	binary.LittleEndian.PutUint32(header[24:], 16000)
	binary.LittleEndian.PutUint32(header[28:], pcmBytesPerSecond)
	binary.LittleEndian.PutUint16(header[32:], 2)
	binary.LittleEndian.PutUint16(header[34:], 16)
	copy(header[36:], "data")
	binary.LittleEndian.PutUint32(header[40:], uint32(len(pcm)))
	return os.WriteFile(path, append(header, pcm...), 0o644)
}
