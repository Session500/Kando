package track

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	"kando/packages/core"
)

// NewID is a new id of a generation or a line, as study.NewID makes them.
func NewID() string {
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}

// Made is how a whole generation was made, which Generation writes into its revisions: what wrote each side's text and at which layer.
type Made struct {
	// Source names what made the generation, as "whisper" or "import".
	Source string
	// Original is the layer of the original lines: LayerHeard as heard, LayerDraft once the machine corrected them, LayerFinal once the film was read through as a whole.
	Original Layer
	// Translation is the layer of the translations, LayerDraft or LayerFinal.
	Translation Layer
}

// Generation is the records of a new generation holding original, the film's lines, with translated their translations line for line (a line left untranslated, or translated empty, has none), done at once: a film made whole, as before its lines could come one by one.
func Generation(made Made, original, translated []core.Segment) []Event {
	gen := NewID()
	events := []Event{{Kind: KindGen, Gen: gen, Source: made.Source}}
	for i, s := range original {
		id := NewID()
		events = append(events,
			Event{Kind: KindLine, Gen: gen, Line: id, Start: s.Start.Milliseconds(), End: s.End.Milliseconds()},
			Event{Kind: KindRev, Line: id, Side: Original, Layer: made.Original, Text: s.Text, Source: made.Source},
		)
		if i < len(translated) {
			if text := strings.TrimSpace(translated[i].Text); text != "" {
				events = append(events, Event{Kind: KindRev, Line: id, Side: Translation, Layer: made.Translation, Text: text, Source: made.Source})
			}
		}
	}
	return append(events, Event{Kind: KindDone, Gen: gen})
}

// Originals are lines as segments of their original text.
func Originals(lines []Line) []core.Segment {
	out := make([]core.Segment, len(lines))
	for i, l := range lines {
		out[i] = core.Segment{Start: l.Start, End: l.End, Text: l.Original.Text}
	}
	return out
}

// Translations are lines as segments of their translation, empty for a line not translated.
func Translations(lines []Line) []core.Segment {
	out := make([]core.Segment, len(lines))
	for i, l := range lines {
		out[i] = core.Segment{Start: l.Start, End: l.End, Text: l.Translation.Text}
	}
	return out
}

// Bilingual are lines as the bilingual subtitles show them: each translation above its original line, or the original alone for a line not translated.
func Bilingual(lines []Line) []core.Segment {
	out := Originals(lines)
	for i, l := range lines {
		if t := strings.TrimSpace(l.Translation.Text); t != "" {
			out[i].Text = t + "\n" + out[i].Text
		}
	}
	return out
}
