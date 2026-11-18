package media

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"kando/packages/core"
)

// proofreadingRedo is redoPipeline with a proofreader that corrects "there" to "their" and counts how often it ran, failing with err when it is set.
type proofreadingRedo struct {
	*redoPipeline
	proofreads int
	err        error
}

func (p *proofreadingRedo) Proofread(_ context.Context, segments []core.Segment, title string) ([]core.Segment, error) {
	p.proofreads++
	if p.err != nil {
		return nil, p.err
	}
	if title != "video" {
		p.t.Errorf("proofread a video called %q, want its file name", title)
	}
	out := make([]core.Segment, len(segments))
	for i, s := range segments {
		s.Text = strings.ReplaceAll(s.Text, "there", "their")
		out[i] = s
	}
	return out, nil
}

// Translating a done film that was never proofread proofreads its transcript first and translates the corrected lines. A run that fails after proofreading leaves the film's subtitles as they were and keeps the proofread lines aside, so the run that follows reads them back rather than proofreading again; once it exports, the film's transcript holds the corrected lines, the lines kept aside are gone and the film is proofread.
func TestRunProofreadsAFilmBeforeTranslatingItAgain(t *testing.T) {
	job := doneFilm(t, "你好。", "", "再见。")
	job.Redo = core.RedoTranslation
	transcript := TranscriptOf(job.VideoPath)
	heard := read(t, transcript)
	p := &proofreadingRedo{redoPipeline: &redoPipeline{t: t, translateErr: errors.New("translation API returned 503")}}

	failed := Run(context.Background(), p, job, "日本語", func(core.Job) {})
	if failed.Stage != core.StageFailed || failed.Proofread || p.proofreads != 1 {
		t.Fatalf("failed %+v, proofread %d times", failed, p.proofreads)
	}
	if read(t, transcript) != heard {
		t.Fatal("a failed run changed the film's transcript")
	}
	if kept, ok := readProofread(job.VideoPath, mustLoad(t, transcript)); !ok || kept[0].Text != "Hello their." {
		t.Fatalf("kept aside %+v, %v", kept, ok)
	}

	p.translateErr = nil
	failed.Stage, failed.Err = core.StageQueued, nil
	done := Run(context.Background(), p, failed, "日本語", func(core.Job) {})
	if done.Stage != core.StageComplete || !done.Proofread || p.proofreads != 1 {
		t.Fatalf("done %+v, proofread %d times", done, p.proofreads)
	}
	if want := []string{"Hello their.", "How are you?", "Goodbye."}; !reflect.DeepEqual(p.sent, want) {
		t.Fatalf("translated %q", p.sent)
	}
	if got := mustLoad(t, transcript); got[0].Text != "Hello their." {
		t.Fatalf("transcript %+v", got)
	}
	if _, err := os.Stat(proofreadTranscriptPath(job.VideoPath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the proofread lines are still kept aside: %v", err)
	}

	// A film that was proofread is not proofread again when it is translated again.
	done.Redo, done.Stage = core.RedoTranslation, core.StageQueued
	if again := Run(context.Background(), p, done, "日本語", func(core.Job) {}); again.Stage != core.StageComplete || p.proofreads != 1 {
		t.Fatalf("again %+v, proofread %d times", again, p.proofreads)
	}
}

// Proofreading that fails, as with a translator that is down, leaves the lines as they were heard and the job goes on to translate them.
func TestRunTranslatesWhatWasHeardWhenProofreadingFails(t *testing.T) {
	job := doneFilm(t, "你好。", "", "再见。")
	job.Redo = core.RedoTranslation
	p := &proofreadingRedo{redoPipeline: &redoPipeline{t: t}, err: errors.New("translation API returned 503")}
	done := Run(context.Background(), p, job, "日本語", func(core.Job) {})
	if done.Stage != core.StageComplete || done.Proofread || p.sent[0] != "Hello there." {
		t.Fatalf("done %+v, translated %q", done, p.sent)
	}
}

// A film heard in the language its subtitles are for is not translated: its subtitles are its own lines alone, none of them counts as untranslated, and the film says so. Translated into another language, it is translated again.
func TestRunLeavesAFilmInItsOwnLanguageUntranslated(t *testing.T) {
	job := doneFilm(t, "你好。", "", "再见。")
	job.Redo, job.SpokenLanguage = core.RedoTranslation, "en"
	p := &redoPipeline{t: t}
	done := Run(context.Background(), p, job, "English", func(core.Job) {})
	if done.Stage != core.StageComplete || !done.SameLanguage || done.Untranslated != 0 || done.TranslationErr != "" || len(p.sent) != 0 {
		t.Fatalf("done %+v, translated %q", done, p.sent)
	}
	if want := []string{"Hello there.", "How are you?", "Goodbye."}; !reflect.DeepEqual(bilingualTexts(t, done), want) {
		t.Fatalf("bilingual %q", bilingualTexts(t, done))
	}

	done.Redo, done.Stage = core.RedoTranslation, core.StageQueued
	again := Run(context.Background(), p, done, "日本語", func(core.Job) {})
	if again.Stage != core.StageComplete || again.SameLanguage || len(p.sent) != 3 {
		t.Fatalf("again %+v, translated %q", again, p.sent)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func mustLoad(t *testing.T, path string) []core.Segment {
	t.Helper()
	lines, err := loadTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	return lines
}
