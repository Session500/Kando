package media

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"kando/packages/core"
	"kando/packages/track"
)

func trackVideo(t *testing.T) (video, transcript, bilingual string) {
	t.Helper()
	dir := t.TempDir()
	video = filepath.Join(dir, "Episode 1.mkv")
	if err := os.WriteFile(video, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return video, filepath.Join(dir, "Episode 1.srt"), filepath.Join(dir, "Episode 1.bilingual.srt")
}

func writeSRT(t *testing.T, path string, cues ...core.Segment) {
	t.Helper()
	if err := os.WriteFile(path, []byte(formatSRT(cues)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func cue(start, end int, text string) core.Segment {
	return core.Segment{Start: time.Duration(start) * time.Second, End: time.Duration(end) * time.Second, Text: text}
}

func readTrack(t *testing.T, video, subtitles, transcript string) []track.Line {
	t.Helper()
	doc, err := ReadTrack(video, subtitles, transcript)
	if err != nil {
		t.Fatal(err)
	}
	return doc.View()
}

func TestReadTrackTakesInTheFilmsSubtitlesWithoutWriting(t *testing.T) {
	video, transcript, bilingual := trackVideo(t)
	writeSRT(t, transcript, cue(1, 2, "おはよう"), cue(3, 4, "またね"))
	writeSRT(t, bilingual, cue(1, 2, "早上好\nおはよう"), cue(3, 4, "またね"))
	lines := readTrack(t, video, bilingual, transcript)
	if len(lines) != 2 || lines[0].Original.Text != "おはよう" || lines[0].Translation.Text != "早上好" || lines[1].Translation.Text != "" {
		t.Fatalf("lines = %+v", lines)
	}
	if lines[0].Original.Layer != track.LayerFinal {
		t.Fatalf("layer = %d, want what a done film's file holds to count as final", lines[0].Original.Layer)
	}
	if got := track.Bilingual(lines); got[0].Text != "早上好\nおはよう" || got[1].Text != "またね" {
		t.Fatalf("Bilingual = %+v, want what the file holds", got)
	}
	if logs := track.Logs(video); logs != nil {
		t.Fatalf("reading wrote %v", logs)
	}
}

func TestReadTrackSplitsCuesByTheTranscript(t *testing.T) {
	video, transcript, bilingual := trackVideo(t)
	// A site's subtitles hold a line of two rows, left untranslated: the bilingual cue alone would read as a translation over its original.
	writeSRT(t, transcript, cue(1, 2, "first row\nsecond row"))
	writeSRT(t, bilingual, cue(1, 2, "first row\nsecond row"))
	lines := readTrack(t, video, bilingual, transcript)
	if len(lines) != 1 || lines[0].Original.Text != "first row\nsecond row" || lines[0].Translation.Text != "" {
		t.Fatalf("lines = %+v, want one original of two rows", lines)
	}
	// Without the transcript the cue is read as the film page reads it.
	lines = readTrack(t, video, bilingual, "")
	if lines[0].Translation.Text != "first row" {
		t.Fatalf("lines = %+v", lines)
	}
	// A film whose bilingual subtitles are not made yet shows its transcript.
	lines = readTrack(t, video, "", transcript)
	if len(lines) != 1 || lines[0].Translation.Text != "" {
		t.Fatalf("lines = %+v", lines)
	}
}

func TestReadTrackKeepsTheLinesOfTheDocument(t *testing.T) {
	video, _, bilingual := trackVideo(t)
	made := track.Generation(track.Made{Source: "whisper", Original: track.LayerHeard, Translation: track.LayerDraft},
		[]core.Segment{cue(1, 2, "hello"), cue(3, 4, "bye")}, []core.Segment{cue(1, 2, "你好"), {}})
	if _, err := track.Append(video, "mac", made...); err != nil {
		t.Fatal(err)
	}
	doc, _ := track.Read(video)
	ids := []string{doc.View()[0].ID, doc.View()[1].ID}
	// The file, as an editor changed it, shows at the lines' times: they keep their ids and take what changed.
	writeSRT(t, bilingual, cue(1, 2, "您好\nhello"), cue(3, 4, "再见\nbye"))
	lines := readTrack(t, video, bilingual, "")
	if got := []string{lines[0].ID, lines[1].ID}; !slices.Equal(got, ids) {
		t.Fatalf("ids = %v, want %v", got, ids)
	}
	if lines[0].Translation.Text != "您好" || lines[1].Translation.Text != "再见" || lines[0].Original.Layer != track.LayerHeard {
		t.Fatalf("lines = %+v", lines)
	}
	// Lines at other times become a generation of their own.
	writeSRT(t, bilingual, cue(1, 3, "一\none"))
	lines = readTrack(t, video, bilingual, "")
	if len(lines) != 1 || lines[0].ID == ids[0] || lines[0].Original.Text != "one" {
		t.Fatalf("lines = %+v", lines)
	}
}

func TestReadTrackOfAFilmWithoutAVideoOrSubtitles(t *testing.T) {
	_, transcript, _ := trackVideo(t)
	if lines := readTrack(t, "", "", transcript); lines != nil {
		t.Fatalf("a film without subtitles shows %+v", lines)
	}
	writeSRT(t, transcript, cue(1, 2, "hello"))
	if lines := readTrack(t, "", transcript, ""); len(lines) != 1 || lines[0].Original.Text != "hello" {
		t.Fatalf("a film without a video shows %+v, want its file", lines)
	}
}

func TestPairCuesKeepsTheTranslationsOfLinesShownAtTheSameTime(t *testing.T) {
	seg := func(start, end int, text string) core.Segment {
		return core.Segment{Start: time.Duration(start) * time.Millisecond, End: time.Duration(end) * time.Millisecond, Text: text}
	}
	line := func(start, end int, original, translation string) core.Line {
		return core.Line{Start: time.Duration(start) * time.Millisecond, End: time.Duration(end) * time.Millisecond, Original: original, Translation: translation}
	}
	original := []core.Segment{seg(0, 1000, "Left"), seg(0, 1000, "Right"), seg(2000, 3000, "Done")}
	bilingual := []core.Segment{seg(0, 1000, "左\nLeft"), seg(0, 1000, "右\nRight"), seg(2000, 3000, "完\nDone")}
	want := []core.Line{line(0, 1000, "Left", "左"), line(0, 1000, "Right", "右"), line(2000, 3000, "Done", "完")}
	if got := PairCues(original, bilingual); !slices.Equal(got, want) {
		t.Fatalf("aligned files: %+v", got)
	}
	// The bilingual file lacks the last line, so the files are not aligned and each line takes the first unused cue shown at its time.
	want[2].Translation = ""
	if got := PairCues(original, bilingual[:2]); !slices.Equal(got, want) {
		t.Fatalf("files of different lengths: %+v", got)
	}
}

func TestCompanionsIncludeTheSubtitlesLogs(t *testing.T) {
	video, _, _ := trackVideo(t)
	if _, err := track.Append(video, "mac", track.Event{Kind: track.KindGone}); err != nil {
		t.Fatal(err)
	}
	if got := Companions(video); !slices.Contains(got, track.LogPath(video, "mac")) {
		t.Fatalf("Companions = %v, want the log", got)
	}
}
