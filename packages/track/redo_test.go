package track

import (
	"slices"
	"testing"
)

// redoDoc is a document of events stamped one after another on one device, as Append would write them.
func redoDoc(events ...Event) Doc {
	for i := range events {
		events[i].Device, events[i].Seq, events[i].At = "mac", int64(i+1), int64(i+1)
	}
	return Doc{Events: events}
}

// redoLine is line id of generation gen, from start to end, in seconds, heard as original and translated as translation ("" for none).
func redoLine(gen, id string, start, end float64, original, translation string) []Event {
	out := []Event{
		{Kind: KindLine, Gen: gen, Line: id, Start: int64(start * 1000), End: int64(end * 1000)},
		{Kind: KindRev, Line: id, Side: Original, Layer: LayerHeard, Text: original},
	}
	if translation != "" {
		out = append(out, Event{Kind: KindRev, Line: id, Side: Translation, Layer: LayerDraft, Text: translation})
	}
	return out
}

// drafted is the record that cell of gen was drafted.
func drafted(gen string, cell int) Event {
	return Unit{Work: WorkDraft, Gen: gen, Cell: cell, Key: "k"}.Made()
}

// shownTexts is each line of lines as original/translation.
func shownTexts(lines []Line) []string {
	var out []string
	for _, l := range lines {
		out = append(out, l.Original.Text+"/"+l.Translation.Text)
	}
	return out
}

// TestARedoShowsItsLinesWhereItHasMadeThem translates a film again into another language and into its own: the cell the redo drafted shows its lines, the cell it has not shows the film's own, without their translation when the film goes into another language now, and with it when it goes into the same one.
func TestARedoShowsItsLinesWhereItHasMadeThem(t *testing.T) {
	events := []Event{{Kind: KindGen, Gen: "old", Source: "make"}}
	events = append(events, redoLine("old", "o1", 1, 3, "Hello.", "你好。")...)
	events = append(events, redoLine("old", "o2", 41, 43, "Bye.", "再见。")...)
	events = append(events, Event{Kind: KindDone, Gen: "old"}, Event{Kind: KindGen, Gen: "new", Source: "make", Key: "copy old"})
	events = append(events, redoLine("new", "n1", 1, 3, "Hello.", "こんにちは。")...)
	events = append(events, redoLine("new", "n2", 41, 43, "Bye.", "")...)
	events = append(events, drafted("new", 0))
	doc := redoDoc(events...)
	if got := shownTexts(doc.Redoing("new", false)); !slices.Equal(got, []string{"Hello./こんにちは。", "Bye./"}) {
		t.Fatalf("translated into another language, the film shows %q", got)
	}
	if got := shownTexts(doc.Redoing("new", true)); !slices.Equal(got, []string{"Hello./こんにちは。", "Bye./再见。"}) {
		t.Fatalf("translated into its own language again, the film shows %q", got)
	}
	// The redo the film does not make, or one done, shows the film as it is.
	if got := shownTexts(doc.Redoing("", false)); !slices.Equal(got, []string{"Hello./你好。", "Bye./再见。"}) {
		t.Fatalf("with no redo the film shows %q", got)
	}
	done := redoDoc(append(slices.Clone(events), drafted("new", 1), Event{Kind: KindDone, Gen: "new"})...)
	if got, want := shownTexts(done.Redoing("new", false)), shownTexts(done.View()); !slices.Equal(got, want) {
		t.Fatalf("once done the film shows %q, its view %q", got, want)
	}
}

// TestARedoCutOtherwiseShowsEachLineOnce processes a film again, its speech heard anew and cut into other lines: a line of the film that a new line of the cell beside it covers half of or more goes, one covered less stays, and a drafted cell the new hearing found silent shows none of the film's lines there.
func TestARedoCutOtherwiseShowsEachLineOnce(t *testing.T) {
	events := []Event{{Kind: KindGen, Gen: "old", Source: "make"}}
	events = append(events, redoLine("old", "o1", 5, 8, "first", "一")...)
	// Its middle, 30.5 s, is in cell 1, which the redo has not drafted; the new line in cell 0 covers 3.5 s of its 5.
	events = append(events, redoLine("old", "o2", 28, 33, "across", "跨")...)
	// Covered 0.5 s of 3 by the new line: it stays.
	events = append(events, redoLine("old", "o3", 31, 34, "after", "后")...)
	events = append(events, redoLine("old", "o4", 65, 67, "silent", "静")...)
	events = append(events, Event{Kind: KindDone, Gen: "old"}, Event{Kind: KindGen, Gen: "new", Source: "make", Key: "large"})
	events = append(events, redoLine("new", "n1", 5, 8, "first anew", "一新")...)
	events = append(events, redoLine("new", "n2", 27, 31.5, "across anew", "跨新")...)
	events = append(events, drafted("new", 0), drafted("new", 2))
	got := shownTexts(redoDoc(events...).Redoing("new", true))
	if want := []string{"first anew/一新", "across anew/跨新", "after/后"}; !slices.Equal(got, want) {
		t.Fatalf("the film shows %q, want %q", got, want)
	}
}
