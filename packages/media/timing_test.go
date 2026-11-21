package media

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"kando/packages/core"
)

// words is a line heard as text, its tokens each timed at the second after it, in the chunk's time.
func words(from, to float64, text string, tokens ...any) decodedLine {
	l := decodedLine{from: sec(from), to: sec(to), text: text}
	for i := 0; i < len(tokens); i += 2 {
		k := decodedToken{text: tokens[i].(string)}
		if at := tokens[i+1].(float64); at >= 0 {
			k.at, k.timed = sec(at), true
		}
		l.tokens = append(l.tokens, k)
	}
	return l
}

func TestTimeLinesStartsALineAtItsFirstWord(t *testing.T) {
	c := chunk{span: span{sec(10), sec(30)}, speech: []span{{sec(10.3), sec(12)}, {sec(13), sec(16)}}}
	heard := []decodedLine{
		words(0.2, 1.8, "Good morning.", "Good", 0.5, " morning", 1.2, ".", 1.6),
		// whisper starts the next line where the last ended, but its words come after the pause.
		words(1.8, 5.8, "How are you?", "How", 3.4, " are", 3.8, " you", 4.4, "?", 5.0),
	}
	got := timeLines(heard, c, true)
	want := []core.Segment{
		// 0.3 s before its first word, but not before the speech it is in starts; 0.6 s after its last word, but not past the speech it is in.
		{Start: sec(10.3), End: sec(11.8), Text: "Good morning."},
		// In the speech after the pause.
		{Start: sec(13.1), End: sec(15), Text: "How are you?"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("lines %+v, want %+v", got, want)
	}
}

func TestTimeLinesCutsALineWhereASentenceEndsAndTheSpeechPauses(t *testing.T) {
	c := chunk{span: span{0, sec(20)}, speech: []span{{sec(1), sec(3)}, {sec(3), sec(6)}}}
	// Two speakers whisper heard as one line: the first sentence ends, and the speech, though without a pause the VAD could hear, starts again.
	heard := []decodedLine{words(1, 6, "Are you there? I am.", "Are", 1.2, " you", 1.6, " there", 2.2, "?", 2.6, " I", 3.4, " am", 4.0, ".", 4.2)}
	got := timeLines(heard, c, true)
	want := []core.Segment{
		{Start: sec(1), End: sec(2.8), Text: "Are you there?"},
		{Start: sec(3.1), End: sec(4.6), Text: "I am."},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("lines %+v, want %+v", got, want)
	}
	// Where the speech goes on through the sentence's end, the line stays one: one speaker says both.
	c.speech = []span{{sec(1), sec(6)}}
	if got := timeLines(heard, c, true); len(got) != 1 || got[0].Text != "Are you there? I am." {
		t.Fatalf("lines %+v", got)
	}
}

func TestTimeLinesCutsALongLineInItsLongestPause(t *testing.T) {
	c := chunk{span: span{0, sec(25)}, speech: []span{{sec(1), sec(4)}, {sec(4.6), sec(7)}, {sec(8.5), sec(12)}}}
	heard := []decodedLine{words(1, 12, "one two three four", "one", 1.5, " two", 5.0, " three", 9.0, " four", 11.5)}
	got := timeLines(heard, c, true)
	// 10 s of words is too long for a line; the longest pause, 1.5 s, cuts it, and each piece is short enough.
	want := []core.Segment{
		{Start: sec(1.2), End: sec(5.6), Text: "one two"},
		{Start: sec(8.7), End: sec(12), Text: "three four"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("lines %+v, want %+v", got, want)
	}
}

func TestTimeLinesDoesNotCutALineItsTokensDoNotSpell(t *testing.T) {
	c := chunk{span: span{0, sec(20)}, speech: []span{{sec(1), sec(3)}, {sec(4), sec(6)}}}
	// An older whisper-cli may write half a character in a token, which reads as U+FFFD.
	heard := []decodedLine{words(1, 6, "Yes. 犬", "Yes", 1.5, ".", 1.8, " �", 4.5, "�", 4.6)}
	got := timeLines(heard, c, true)
	if len(got) != 1 || got[0].Text != "Yes. 犬" || got[0].Start != sec(1.2) {
		t.Fatalf("lines %+v", got)
	}
}

func TestTimeLinesUsesWhispersTimesWithoutWordTimes(t *testing.T) {
	c := chunk{span: span{sec(10), sec(20)}, speech: []span{{sec(10.3), sec(12)}, {sec(15), sec(19.7)}}}
	heard := []decodedLine{words(2.5, 8, "Later.", "Later", 6.0, ".", 6.5)}
	// Without DTW the tokens are not timed: whisper's own times are placed (placeLine).
	got := timeLines(heard, c, false)
	if want := (core.Segment{Start: sec(15), End: sec(18), Text: "Later."}); len(got) != 1 || got[0] != want {
		t.Fatalf("lines %+v, want %v", got, want)
	}
}

func TestTimeLinesKeepsALineUpLongEnoughToRead(t *testing.T) {
	c := chunk{span: span{0, sec(20)}, speech: []span{{sec(1), sec(10)}}}
	heard := []decodedLine{
		words(1, 1.5, "Oh.", "Oh", 1.4, ".", 1.5),
		words(1.5, 3, "Well.", "Well", 1.9, ".", 2.0),
		words(3, 6, "So.", "So", 5.0, ".", 5.1),
	}
	got := timeLines(heard, c, true)
	// "Oh." stays up until the next line starts, "Well." a second though the next starts later.
	want := []core.Segment{
		{Start: sec(1.1), End: sec(1.6), Text: "Oh."},
		{Start: sec(1.6), End: sec(2.6), Text: "Well."},
		{Start: sec(4.7), End: sec(5.7), Text: "So."},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("lines %+v, want %+v", got, want)
	}
}

func TestCollapseRepeatsMakesALineSaidOverAndOverOne(t *testing.T) {
	heard := []decodedLine{
		words(0, 1, "a", "a", 0.5),
		words(1, 2.3, "b", "b", 1.5),
		words(2.3, 2.36, "b", "b", 2.33),
		words(2.36, 4, "b", "b", 3.5),
		words(5, 6, "a", "a", 5.5),
	}
	got := collapseRepeats(heard)
	if len(got) != 3 || got[1].text != "b" || got[1].to != sec(4) || got[1].until != sec(3.5) || got[2].text != "a" {
		t.Fatalf("collapsed to %+v", got)
	}
	// The line said over and over ends after the last time it was said.
	c := chunk{span: span{0, sec(10)}, speech: []span{{0, sec(10)}}}
	if lines := timeLines(heard, c, true); len(lines) != 3 || lines[1].End != sec(4.1) {
		t.Fatalf("lines %+v", lines)
	}
}

func TestParseHeardReadsWordTimesAndDropsControlCharacters(t *testing.T) {
	// whisper-cli writes a control character in a line as it is, which JSON does not allow.
	data := []byte("{\"transcription\":[{\"offsets\":{\"from\":100,\"to\":900},\"text\":\" a\x01b\",\"tokens\":[{\"text\":\"[_BEG_]\",\"t_dtw\":-1},{\"text\":\" a\x01b\",\"t_dtw\":42}]},{\"offsets\":{\"from\":900,\"to\":900},\"text\":\" \"}]}")
	got, err := parseHeard(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].text != "ab" || got[0].from != sec(0.1) || len(got[0].tokens) != 2 || got[0].tokens[0].timed || !got[0].tokens[1].timed || got[0].tokens[1].at != sec(0.42) {
		t.Fatalf("heard %+v", got)
	}
}

func TestDTWPresetIsNamedByTheModelsSizes(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name   string
		header []int32
		want   string
	}{
		{"base", []int32{51865, 1500, 512, 8, 6, 448, 512, 8, 6, 80, 1}, "base"},
		{"tiny.en", []int32{51864, 1500, 384, 6, 4, 448, 384, 6, 4, 80, 1}, "tiny.en"},
		{"large-v3-turbo", []int32{51866, 1500, 1280, 20, 32, 448, 1280, 20, 4, 128, 8}, "large.v3.turbo"},
		{"large-v3", []int32{51866, 1500, 1280, 20, 32, 448, 1280, 20, 32, 128, 1}, "large.v3"},
		// large-v1 and large-v2 have the same sizes and different alignment heads.
		{"large-v2", []int32{51865, 1500, 1280, 20, 32, 448, 1280, 20, 32, 80, 1}, ""},
	} {
		data := append([]byte("lmgg"), make([]byte, 4*len(tc.header))...)
		for i, v := range tc.header {
			binary.LittleEndian.PutUint32(data[4+4*i:], uint32(v))
		}
		path := filepath.Join(dir, tc.name+".bin")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if got := dtwPreset(path); got != tc.want {
			t.Errorf("%s: preset %q, want %q", tc.name, got, tc.want)
		}
	}
	if got := dtwPreset(filepath.Join(dir, "missing.bin")); got != "" {
		t.Fatalf("preset of a missing model %q", got)
	}
}
