package media

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kando/packages/core"
)

func TestMadeUpIsWhatNoVoiceThereSays(t *testing.T) {
	noVoice := map[string]bool{voiceless("Thanks for watching!"): true}
	line := func(text string, d float64) core.Segment {
		return core.Segment{Start: sec(10), End: sec(10 + d), Text: text}
	}
	for _, tc := range []struct {
		name string
		s    core.Segment
		want bool
	}{
		{"a line said in its time", line("Is anyone there?", 1.2), false},
		{"nothing said", line("- ...", 1), true},
		{"a single syllable, as whisper writes over music or a gasp", line("Oh!", 1), true},
		{"only a sound named", line("[Music] *sighs* ♪", 2), true},
		{"a line with a sound named in it", line("(laughs) You came back.", 2), false},
		{"more said than a voice says in its time", line("I would like a cup of tea, please", 0.4), true},
		{"what whisper says where no one speaks, but for its marks", line("thanks for watching", 1.5), true},
		{"no time to say it in", line("Hello", 0), true},
	} {
		if got := madeUp(tc.s, noVoice); got != tc.want {
			t.Errorf("%s: made up = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestNoVoiceSoundsAreTheSameEachTime(t *testing.T) {
	a, b := noVoiceSounds(), noVoiceSounds()
	if len(a) < 20 || len(a) != len(b) {
		t.Fatalf("%d and %d sounds", len(a), len(b))
	}
	var length time.Duration
	for i := range a {
		if string(a[i]) != string(b[i]) {
			t.Fatalf("sound %d differs", i)
		}
		length += pcmLength(a[i])
	}
	// About four minutes of sound, which whisper hears in ten seconds or so.
	if length < 3*time.Minute || length > 5*time.Minute {
		t.Fatalf("%v of sound", length)
	}
}

func TestHearLeavesOutWhatWhisperSaysWhereOnlyTheQuietVADFindsSpeech(t *testing.T) {
	// The VAD is sure of speech at 3–5 s; less sure, it finds it to 5.5 s, and at 10–11 s. whisper says the same in each chunk, and says it over sound with no voice in it too.
	tools := speechTools(t, fakeTool{Speech: [][2]float64{{3, 5}}, Quiet: [][2]float64{{3, 5.5}, {10, 11}}, Says: "Thank you.", NoVoice: "Thank you."})
	if err := os.MkdirAll(filepath.Dir(tools.WhisperModel), 0o755); err != nil {
		t.Fatal(err)
	}
	var lines []core.Segment
	if _, err := tools.Hear(withSpeechLanguage(context.Background(), "en"), filepath.Join(t.TempDir(), "film.mkv"), 0, 30*time.Second, func(s core.Segment) { lines = append(lines, s) }); err != nil {
		t.Fatal(err)
	}
	// Said where the VAD is sure of speech, the line is kept, as a voice there may say it; where only the quiet VAD finds speech, it is what whisper says where no one does.
	if len(lines) != 1 || lines[0].Start >= sec(5) {
		t.Fatalf("lines %+v", lines)
	}
	// What whisper says where no one speaks is kept beside the model, for the next run.
	saved, _ := filepath.Glob(filepath.Join(filepath.Dir(tools.WhisperModel), "novoice", "en-*.json"))
	if len(saved) != 1 {
		t.Fatalf("saved %v", saved)
	}
	if data, _ := os.ReadFile(saved[0]); string(data) != `["thankyou"]` {
		t.Fatalf("saved %s", data)
	}
}
