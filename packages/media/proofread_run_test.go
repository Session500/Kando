package media

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

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

// A film translated again keeps its lines as the film shows them: they are not proofread again, which a film made again entirely (core.RedoAll) is.
func TestMakeTranslatingAgainDoesNotProofread(t *testing.T) {
	job := doneFilm(t, "你好。", "", "再见。")
	job.Redo = core.RedoTranslation
	p := &proofreadingRedo{redoPipeline: &redoPipeline{t: t}}
	done, _ := remake(p, job, "日本語")
	if done.Stage != core.StageComplete || p.proofreads != 0 {
		t.Fatalf("done %+v, proofread %d times", done, p.proofreads)
	}
	if want := []string{"Hello there.", "How are you?", "Goodbye."}; !reflect.DeepEqual(p.sent, want) {
		t.Fatalf("translated %q", p.sent)
	}
}

// failingProofreader is fakePipeline with a proofreader that is down.
type failingProofreader struct{ fakePipeline }

func (p *failingProofreader) setDir(dir string) { p.dir = dir }

func (p *failingProofreader) Proofread(context.Context, []core.Segment, string) ([]core.Segment, error) {
	return nil, errors.New("translation API returned 503")
}

// Proofreading that fails, as with a translator that is down, leaves the lines as they were heard and the film goes on to translate them.
func TestMakeTranslatesWhatWasHeardWhenProofreadingFails(t *testing.T) {
	p := &failingProofreader{fakePipeline{transcribe: []core.Segment{{End: time.Second, Text: "Hello there."}}}}
	_, done := runStages(t, p)
	if done.Stage != core.StageComplete || done.Untranslated != 0 {
		t.Fatalf("done %+v", done)
	}
	if want := []string{"Hello there.\nHello there."}; !reflect.DeepEqual(bilingualTexts(t, done), want) {
		t.Fatalf("bilingual %q", bilingualTexts(t, done))
	}
}

// A film heard in the language its subtitles are for is not translated: its subtitles are its own lines alone, none of them counts as untranslated, and the film says so. Translated into another language, it is translated again.
func TestMakeLeavesAFilmInItsOwnLanguageUntranslated(t *testing.T) {
	job := doneFilm(t, "你好。", "", "再见。")
	job.Redo, job.SpokenLanguage = core.RedoTranslation, "en"
	p := &redoPipeline{t: t}
	done, _ := remake(p, job, "English")
	if done.Stage != core.StageComplete || !done.SameLanguage || done.Untranslated != 0 || done.TranslationErr != "" || len(p.sent) != 0 {
		t.Fatalf("done %+v, translated %q", done, p.sent)
	}
	if want := []string{"Hello there.", "How are you?", "Goodbye."}; !reflect.DeepEqual(bilingualTexts(t, done), want) {
		t.Fatalf("bilingual %q", bilingualTexts(t, done))
	}

	done.Redo, done.Stage = core.RedoTranslation, core.StageQueued
	again, _ := remake(p, done, "日本語")
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
