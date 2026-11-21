package media

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"kando/packages/core"
)

func sec(s float64) time.Duration { return seconds(s) }

func TestPackChunksHearsSpeechCloseTogetherAsOne(t *testing.T) {
	spans := []span{{sec(1), sec(2)}, {sec(3), sec(4)}, {sec(10), sec(11)}, {sec(11.5), sec(40)}}
	got := packChunks(spans, span{0, sec(50)})
	// The first two are a second apart and heard together; the third is close to the fourth, but the two together would be longer than a chunk may be, so each is padded only halfway to the other.
	want := []span{{sec(0.7), sec(4.3)}, {sec(9.7), sec(11.25)}, {sec(11.25), sec(40.3)}}
	if len(got) != len(want) {
		t.Fatalf("chunks %+v", got)
	}
	for i, c := range got {
		if c.span != want[i] {
			t.Fatalf("chunk %d = %v, want %v", i, c.span, want[i])
		}
	}
	if len(got[0].speech) != 2 || len(got[2].speech) != 1 {
		t.Fatalf("speech of the chunks %+v", got)
	}
	// Chunks keep within the bounds they are packed in, as the stretch of a run they were found in.
	if got := packChunks([]span{{sec(1), sec(2)}}, span{sec(1.2), sec(1.9)}); got[0].span != (span{sec(1.2), sec(1.9)}) {
		t.Fatalf("chunk %v beyond its bounds", got[0].span)
	}
}

func TestPlaceLineStartsItWhereItsSpeechDoes(t *testing.T) {
	c := chunk{span: span{sec(10), sec(20)}, speech: []span{{sec(10.3), sec(12)}, {sec(15), sec(19.7)}}}
	for _, tc := range []struct {
		name       string
		start, end float64
		want       [2]float64
	}{
		{"in its speech", 0.5, 1.8, [2]float64{10.5, 11.8}},
		// whisper starts the line after the pause in the pause.
		{"in the pause before its speech", 2.5, 8, [2]float64{15, 18}},
		{"in a pause with no speech after it", 9.8, 12, [2]float64{19.8, 20}},
	} {
		s := placeLine(core.Segment{Start: sec(tc.start), End: sec(tc.end)}, c)
		if s.Start != sec(tc.want[0]) || s.End != sec(tc.want[1]) {
			t.Errorf("%s: %v-%v, want %v", tc.name, s.Start, s.End, tc.want)
		}
	}
}

func TestPauseNearCutsAStretchInTheNearestPause(t *testing.T) {
	spans := []span{{sec(1), sec(30)}, {sec(40), sec(44)}, {sec(48), sec(55)}}
	if cut := pauseNear(spans, sec(45), sec(55)); cut != sec(46) {
		t.Fatalf("cut at %v, want in the pause from 44 s to 48 s", cut)
	}
	if got := before(spans, sec(46)); len(got) != 2 || got[1] != (span{sec(40), sec(44)}) {
		t.Fatalf("speech before the cut %+v", got)
	}
	// Speech on to the end, with no pause: the stretch ends where it was searched to.
	if cut := pauseNear([]span{{0, sec(55)}}, sec(45), sec(55)); cut != sec(55) {
		t.Fatalf("cut at %v", cut)
	}
}

func TestReadPCMFindsTheSamplesAfterOtherChunks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.wav")
	if err := writePCM(path, []byte{1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	pcm, err := readPCM(path)
	if err != nil || !slices.Equal(pcm, []byte{1, 2, 3, 4}) {
		t.Fatalf("read %v (%v)", pcm, err)
	}
	// ffmpeg may write a LIST chunk before the samples.
	data, _ := os.ReadFile(path)
	list := []byte("LIST\x05\x00\x00\x00abcde\x00")
	withList := append(append(append([]byte{}, data[:36]...), list...), data[36:]...)
	binary.LittleEndian.PutUint32(withList[4:], uint32(len(withList)-8))
	if err := os.WriteFile(path, withList, 0o644); err != nil {
		t.Fatal(err)
	}
	if pcm, err := readPCM(path); err != nil || !slices.Equal(pcm, []byte{1, 2, 3, 4}) {
		t.Fatalf("read %v (%v) past a LIST chunk", pcm, err)
	}
}

func TestFillRepeatsTheSoundToTheLength(t *testing.T) {
	pcm := make([]byte, pcmBytesPerSecond) // a second
	pcm[0] = 7
	got := fill(pcm, 2500*time.Millisecond)
	if pcmLength(got) != 2500*time.Millisecond || got[pcmBytesPerSecond] != 7 || got[2*pcmBytesPerSecond] != 7 {
		t.Fatalf("filled to %v", pcmLength(got))
	}
}

func TestMostOfIsTheLanguageToldMostOften(t *testing.T) {
	if got := mostOf([]string{"ko", "ja", "ja"}); got != "ja" {
		t.Fatalf("most of = %q", got)
	}
	if got := mostOf(nil); got != "" {
		t.Fatalf("most of none = %q", got)
	}
}

// speechTools is Tools whose ffmpeg writes silence as long as it is asked for and whose whisper-cli finds speech as fake says.
func speechTools(t *testing.T, fake fakeTool) Tools {
	t.Helper()
	dir := t.TempDir()
	fake.Act = "speech"
	return Tools{
		FFmpeg:       writeFakeTool(t, dir, "ffmpeg", fakeTool{Act: "extract-wav"}),
		Whisper:      writeFakeTool(t, dir, "whisper-cli", fake),
		WhisperModel: filepath.Join(dir, "models", "ggml-base.bin"),
	}
}

func TestHearHearsOnlyTheSpeechAChunkAtATime(t *testing.T) {
	tools := speechTools(t, fakeTool{Speech: [][2]float64{{3, 5}, {6, 7}, {20, 30}}, Told: []string{"ko", "ja", "ja"}})
	if err := os.MkdirAll(filepath.Dir(tools.WhisperModel), 0o755); err != nil {
		t.Fatal(err)
	}
	var lines []core.Segment
	language, err := tools.Hear(context.Background(), filepath.Join(t.TempDir(), "film.mkv"), time.Minute, 90*time.Second, func(s core.Segment) { lines = append(lines, s) })
	if err != nil {
		t.Fatal(err)
	}
	// The run has 13 s of speech, too little, so the speech after it is listened for too; of the six windows of speech, the first told as Korean, most are told as Japanese.
	if language != "ja" {
		t.Fatalf("language %q", language)
	}
	// Speech a second apart is heard as one chunk, from 2.7 s, and the speech 13 s later as another, to the end of the sound; each line is placed in the film where its speech starts.
	want := []core.Segment{
		{Start: 63 * time.Second, End: 63700 * time.Millisecond, Text: "chunk-0-0000.wav"},
		{Start: 80 * time.Second, End: 80700 * time.Millisecond, Text: "chunk-0-0001.wav"},
	}
	if !slices.Equal(lines, want) {
		t.Fatalf("lines %+v, want %+v", lines, want)
	}
	// The VAD model Kando carries is written beside the speech model.
	if info, err := os.Stat(filepath.Join(filepath.Dir(tools.WhisperModel), sileroFile)); err != nil || info.Size() != int64(len(sileroModel)) {
		t.Fatalf("VAD model %v (%v)", info, err)
	}
}

func TestHearHearsTheWholeSoundWhenTheSpeechCannotBeFound(t *testing.T) {
	tools := speechTools(t, fakeTool{NoVAD: true, Transcript: `{"result":{"language":"ja"},"transcription":[]}`})
	var lines []string
	language, err := tools.Hear(withSpeechLanguage(context.Background(), "ja"), filepath.Join(t.TempDir(), "film.mkv"), 0, 30*time.Second, func(s core.Segment) { lines = append(lines, s.Text) })
	if err != nil || language != "ja" || !slices.Equal(lines, []string{"heard whole"}) {
		t.Fatalf("heard %q in %q (%v)", lines, language, err)
	}
}

func TestCollapseRepeatsMakesALineSaidOverAndOverOne(t *testing.T) {
	lines := []core.Segment{
		{Start: sec(0), End: sec(1), Text: "a"},
		{Start: sec(1), End: sec(2.3), Text: "b"},
		{Start: sec(2.3), End: sec(2.36), Text: "b"},
		{Start: sec(2.36), End: sec(4), Text: "b"},
		{Start: sec(5), End: sec(6), Text: "a"},
	}
	want := []core.Segment{
		{Start: sec(0), End: sec(1), Text: "a"},
		{Start: sec(1), End: sec(4), Text: "b"},
		{Start: sec(5), End: sec(6), Text: "a"},
	}
	if got := collapseRepeats(lines); !slices.Equal(got, want) {
		t.Fatalf("collapsed to %+v", got)
	}
}
