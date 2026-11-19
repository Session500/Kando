package media

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"kando/packages/core"
	"kando/packages/track"
)

// Maker is a pipeline that makes a film's subtitles a part at a time (Make): it hears part of the video at a time (Hearer) and tells how long the video is.
type Maker interface {
	Pipeline
	Hearer
	Length(ctx context.Context, videoPath string) time.Duration
}

// Length is how long the video at videoPath is, as ffmpeg reads it; 0 when it cannot tell.
func (t Tools) Length(_ context.Context, videoPath string) time.Duration {
	return videoDuration(t.FFmpeg, videoPath)
}

// Watcher says where a film is watched while it is made (Make).
type Watcher interface {
	// Playhead is where the film is watched now, or will be once it is opened: the lines nearest it after it are made first.
	Playhead() time.Duration
	// Jumped receives when the playhead jumped, so the work far from it gives way to the work near it.
	Jumped() <-chan struct{}
}

// From is a Watcher of a film not watched yet, which will be watched from at.
type From time.Duration

func (f From) Playhead() time.Duration { return time.Duration(f) }
func (From) Jumped() <-chan struct{}   { return nil }

// Making a film a part at a time: how much of it one run of the speech recognition hears, and how much sound before and after that run hears too, so a line cut by the run's ends is heard whole by one of the runs on either side, which keeps it where its middle falls (track.Cell).
const (
	runCells = 10
	runPad   = 15 * time.Second
	// draftContext is how many lines said before a cell go with its draft, as the translation sends the lines before a batch (contextLines).
	draftContext = 80
	// projectEvery is how often the subtitle files beside the video are written out while the film is made, at most.
	projectEvery = 3 * time.Second
)

// minRun is how long a run of the speech recognition goes before a jump may stop it, so jumping back and forth does not stop every run before it hears anything; tests shorten it.
var minRun = 10 * time.Second

// Make makes the subtitles of a film for the first time a part at a time, as its job asks (core.Job.Redo empty), into the film's subtitle document (track), device's log, and writes them out beside the video as they come (the transcript and "<video>.bilingual.srt"), so the film can be watched while it is made. The film's cells are heard and drafted nearest the playhead watch says first (track.Doc.Plan): the speech recognition hears a run of cells at a time, and the translation drafts each cell once heard, the two at once. A job run again, after it was stopped, failed or the app quit, goes on with what its document lacks.
//
// A job that cannot be made a part at a time is run as Run runs it: a redo, a link whose lines come from its site's subtitles or that keeps no video, a pipeline that is not a Maker, a video whose length cannot be told, or one whose document a newer Kando wrote.
func Make(ctx context.Context, p Pipeline, job core.Job, language, device string, watch Watcher, report func(core.Job)) core.Job {
	maker, ok := p.(Maker)
	if !ok || job.Redo != "" || job.Quality == core.QualitySubtitles || job.SubtitleSource == core.SubtitlesPlatform || device == "" {
		return Run(ctx, p, job, language, report)
	}
	if watch == nil {
		watch = From(job.ResumeAt)
	}
	enter := func(stage core.Stage) {
		job.Stage = stage
		job.Progress = core.Progress{}
		report(job)
	}
	fail := func(step string, err error) core.Job {
		job.Err = fmt.Errorf("%s: %w", step, err)
		enter(core.StageFailed)
		return job
	}
	var chapters []core.Chapter
	ctx = WithTrace(ctx, &Trace{
		Info: func(info Info) {
			if info.Title == "" || (info.Title == job.Title && info.Thumbnail == job.Thumbnail) {
				return
			}
			job.Title, job.Thumbnail = info.Title, info.Thumbnail
			report(job)
		},
		Chapters: func(list []core.Chapter) { chapters = list },
	})
	if code := strings.TrimSpace(job.SpeechLanguage); code != "" {
		ctx = withSpeechLanguage(ctx, code)
	}
	ctx = withQuality(ctx, job.Quality)
	if err := p.Check(); err != nil {
		return fail("check", err)
	}
	_, local := localVideoPath(job.Source)
	video := job.VideoPath
	if local || !Downloaded(video) {
		if !local {
			enter(core.StageDownloading)
		}
		var err error
		if video, err = p.Download(ctx, job.Source); err != nil {
			job.FailKind = downloadFailKind(job, err)
			return fail("download", err)
		}
	}
	job.VideoPath = video
	if len(chapters) > 0 {
		keepChapters(video, chapters, job.ChapterUse)
	}
	length := maker.Length(ctx, video)
	doc, err := track.Read(video)
	if length <= 0 || err != nil || doc.Newer {
		return Run(ctx, p, job, language, report)
	}
	m := &making{ctx: ctx, p: maker, job: &job, video: video, device: device, doc: doc, report: report}
	return m.run(length, language, watch)
}

// making is a film being made by Make.
type making struct {
	ctx    context.Context
	p      Maker
	job    *core.Job
	video  string
	device string
	doc    track.Doc
	report func(core.Job)
	// transcript and bilingual are the files the film's lines are written out to (project), and projected when they last were.
	transcript, bilingual string
	projected             time.Time
}

// heardCell is a cell a run of the speech recognition heard, with its lines.
type heardCell struct {
	cell  int
	lines []core.Segment
}

// hearDone is how a run of the speech recognition ended: the language it heard, and why it failed, with whether it had heard any line when it did.
type hearDone struct {
	language string
	err      error
	heard    bool
}

// drafted is how a cell's draft ended: the unit, its translations line for line, and why it failed.
type drafted struct {
	unit  track.Unit
	ids   []string
	lines []core.Segment
	err   error
}

func (m *making) run(length time.Duration, language string, watch Watcher) core.Job {
	job := m.job
	// Whatever is still running when the making ends, a run of the speech recognition or a draft, stops with it.
	var stop context.CancelFunc
	m.ctx, stop = context.WithCancel(m.ctx)
	defer stop()
	gen, err := m.generation()
	if err != nil {
		return m.fail("transcribe", err)
	}
	m.transcript = TranscriptOf(m.video)
	if _, local := localVideoPath(job.Source); local && !job.Transcribed && foreignTranscript(m.video) {
		m.transcript = asideTranscriptPath(m.video)
	}
	m.bilingual = strings.TrimSuffix(m.video, filepath.Ext(m.video)) + ".bilingual.srt"

	// The language the speech is heard in: the one the user named for the film, else the one Tools names, else the one an earlier run heard. Without one, the first run hears a cell alone to find it, and every cell after is heard in it.
	hearing := strings.TrimSpace(job.SpeechLanguage)
	if h, ok := m.p.(hearer); ok && hearing == "" {
		_, hearing = h.hearing(m.ctx)
	}
	if hearing == "" {
		hearing = job.SpokenLanguage
	}
	if hearing != "" {
		m.ctx = withSpeechLanguage(m.ctx, hearing)
		job.SpokenLanguage = cmpOr(job.SpokenLanguage, hearing)
	}
	plan := track.Making{Gen: gen, Length: length, Playhead: watch.Playhead(), Hearing: cmpOr(hearing, "auto")}

	// notTranslated is why the film is not translated, once it is not: no translator set up, the service gone, or failing cell after cell.
	var notTranslated string
	if tc, ok := m.p.(TranslateChecker); ok {
		if err := tc.CheckTranslate(); err != nil {
			notTranslated = err.Error()
		}
	}
	// translates is the language the film's lines are drafted in, "" while they are not: a film heard in the language its subtitles are for is not translated, which for Chinese is told by the script of the lines heard so far.
	translates := func() string {
		if notTranslated != "" || hearing == "" || core.SameLanguage(job.SpokenLanguage, language, track.Originals(m.doc.Lines(gen))) {
			return ""
		}
		return language
	}
	plan.Language = translates()

	heardCh := make(chan heardCell)
	doneCh := make(chan hearDone, 1)
	draftCh := make(chan drafted, 1)
	var hearStop context.CancelFunc
	var hearCells []int
	var hearStarted time.Time
	// pending are the cells the first run heard while the language was not known yet, kept until it is, so they are kept as heard in it.
	var pending []heardCell
	drafting := false
	draftFailures := 0
	defer func() {
		if hearStop != nil {
			hearStop()
		}
	}()
	for {
		plan.Playhead = watch.Playhead()
		p := m.doc.Plan(plan)
		m.progress(p)
		if p.Done() && hearStop == nil && !drafting {
			break
		}
		// A run hearing far from where the film is now watched gives way, once it has gone long enough to have heard something; what it heard is kept.
		if hearStop != nil && len(p.Hear) > 0 && !containsCell(hearCells, p.Hear[0].Cell) && time.Since(hearStarted) >= minRun {
			hearStop()
		}
		if hearStop == nil && len(p.Hear) > 0 {
			hearCells = runOf(p.Hear, hearing != "")
			var runCtx context.Context
			runCtx, hearStop = context.WithCancel(m.ctx)
			hearStarted = time.Now()
			go m.hear(runCtx, hearCells, heardCh, doneCh)
		}
		if !drafting && len(p.Draft) > 0 {
			drafting = true
			go m.draft(m.ctx, m.draftOf(p.Draft[0]), plan.Language, draftCh)
		}
		if hearStop == nil && !drafting {
			// Nothing runs and nothing can start, yet the film is not done. Plan says no such state exists; stop rather than wait for ever.
			return m.fail("transcribe", errors.New("subtitles: work is left that nothing can do"))
		}
		select {
		case <-m.ctx.Done():
			return m.fail("transcribe", m.ctx.Err())
		case <-watch.Jumped():
		case c := <-heardCh:
			if hearing == "" {
				pending = append(pending, c)
				continue
			}
			m.commitHeard(gen, plan.Hearing, c)
		case d := <-doneCh:
			hearStop()
			hearStop = nil
			switch {
			case d.err != nil && m.ctx.Err() != nil:
			case errors.Is(d.err, context.Canceled):
				// Stopped for a jump: the cells it did not hear are heard again.
			case d.err != nil && !d.heard:
				// A run that failed before it heard anything, as one whose model cannot load, fails every run: the job fails, as the transcription failing does.
				return m.fail("transcribe", d.err)
			case d.err != nil:
				m.append(track.Unit{Work: track.WorkHear, Gen: gen, Cell: m.firstUnheard(gen, plan.Hearing, hearCells), Key: plan.Hearing}.Failed(d.err.Error()))
			case hearing == "":
				// The first run heard the language: the cells it heard are kept as heard in it, and every cell after is heard in it.
				hearing = cmpOr(d.language, "auto")
				job.SpokenLanguage = d.language
				if d.language != "" {
					m.ctx = withSpeechLanguage(m.ctx, d.language)
				}
				plan.Hearing = hearing
				for _, c := range pending {
					m.commitHeard(gen, hearing, c)
				}
				pending = nil
				plan.Language = translates()
			}
			if hearing == "" {
				pending = nil
			}
		case d := <-draftCh:
			drafting = false
			switch {
			case d.err == nil:
				draftFailures = 0
				m.commitDraft(d)
			case m.ctx.Err() != nil:
			case errors.Is(d.err, ErrOffline) || draftFailures+1 >= maxFailedBatches:
				// The service is not there, or fails cell after cell: the film is made without the translation of the cells left, as it is without a translator.
				notTranslated = d.err.Error()
				plan.Language = translates()
			default:
				draftFailures++
				m.append(d.unit.Failed(d.err.Error()))
			}
		}
		m.project(false)
	}

	lines := m.doc.Lines(gen)
	if len(lines) == 0 {
		return m.fail("transcribe", ErrNoSpeech)
	}
	m.append(track.Event{Kind: track.KindDone, Gen: gen})
	m.project(true)
	same := core.SameLanguage(job.SpokenLanguage, language, track.Originals(lines))
	untranslated := 0
	if !same {
		for _, l := range lines {
			if l.Translation.Text == "" {
				untranslated++
			}
		}
	}
	if notTranslated == "" && !same {
		if err := translateChapters(m.ctx, m.p, m.video, language, same); err != nil {
			return m.fail("translate", err)
		}
	}
	job.Transcribed, job.Proofread, job.SameLanguage = true, false, same
	job.PlatformSubtitles, job.LocalLines, job.SubtitlesOnly = false, 0, false
	job.SubtitlePath = m.bilingual
	job.CompletedAt = time.Now()
	job.Language, job.Untranslated, job.TranslationErr = language, untranslated, notTranslated
	job.Progress = core.Progress{}
	job.Stage = core.StageComplete
	m.report(*job)
	return *job
}

func (m *making) fail(step string, err error) core.Job {
	m.job.Err = fmt.Errorf("%s: %w", step, err)
	m.job.Stage = core.StageFailed
	m.job.Progress = core.Progress{}
	m.report(*m.job)
	return *m.job
}

// generation is the generation a first making of the film makes: the one an earlier run began and did not finish, or a new one.
func (m *making) generation() (string, error) {
	var open track.Event
	for _, e := range m.doc.Events {
		if e.Kind == track.KindGen && e.Source == makeSource && !m.doc.Done(e.Gen) {
			open = e
		}
	}
	if open.Gen != "" {
		return open.Gen, nil
	}
	gen := track.NewID()
	return gen, m.append(track.Event{Kind: track.KindGen, Gen: gen, Source: makeSource})
}

// makeSource is the Source of what Make writes.
const makeSource = "make"

// append writes events to the device's log and keeps them in the document.
func (m *making) append(events ...track.Event) error {
	written, err := track.Append(m.video, m.device, events...)
	m.doc.Events = append(m.doc.Events, written...)
	return err
}

// hear hears cells, a run of consecutive cells, handing each on with its lines once a line past it is heard, or the run ends, then how the run ended.
func (m *making) hear(ctx context.Context, cells []int, heardCh chan<- heardCell, doneCh chan<- hearDone) {
	first, last := cells[0], cells[len(cells)-1]
	from := time.Duration(first) * track.Cell
	to := time.Duration(last+1) * track.Cell
	buffers := map[int][]core.Segment{}
	next := first
	heard := false
	send := func(upTo int) {
		for ; next <= last && next < upTo; next++ {
			select {
			case heardCh <- heardCell{cell: next, lines: buffers[next]}:
			case <-ctx.Done():
				return
			}
		}
	}
	language, err := m.p.Hear(ctx, m.video, max(from-runPad, 0), to+runPad, func(s core.Segment) {
		// A line heard, wherever it is, says the speech recognition runs: a run that fails after it fails a cell, not every run.
		heard = true
		mid := (s.Start + s.End) / 2
		if mid < from || mid >= to {
			return
		}
		cell := int(mid / track.Cell)
		buffers[cell] = append(buffers[cell], s)
		// Lines come in the order they are said: a line in this cell means every cell before it is heard whole.
		send(cell)
	})
	if err == nil {
		send(last + 1)
	}
	doneCh <- hearDone{language: language, err: err, heard: heard}
}

// commitHeard writes a cell's lines, then that it is heard, at once, so a run stopped never leaves part of a cell.
func (m *making) commitHeard(gen, hearing string, c heardCell) {
	var events []track.Event
	for _, s := range c.lines {
		id := track.NewID()
		events = append(events,
			track.Event{Kind: track.KindLine, Gen: gen, Line: id, Start: s.Start.Milliseconds(), End: s.End.Milliseconds()},
			track.Event{Kind: track.KindRev, Line: id, Side: track.Original, Layer: track.LayerHeard, Text: s.Text, Source: makeSource},
		)
	}
	events = append(events, track.Unit{Work: track.WorkHear, Gen: gen, Cell: c.cell, Key: hearing}.Made())
	_ = m.append(events...)
}

// firstUnheard is the first of cells not heard yet, the one a run that failed was hearing.
func (m *making) firstUnheard(gen, hearing string, cells []int) int {
	p := m.doc.Plan(track.Making{Gen: gen, Length: time.Duration(cells[len(cells)-1]+1) * track.Cell, Hearing: hearing})
	for _, c := range cells {
		for _, u := range p.Hear {
			if u.Cell == c {
				return c
			}
		}
	}
	return cells[0]
}

// draftInput is what a cell's draft translates: its lines, by id, and the lines said before them.
type draftInput struct {
	unit    track.Unit
	ids     []string
	lines   []core.Segment
	earlier []string
}

// draftOf is what unit's draft translates, read from the document by the making's own goroutine, so the draft running beside it never reads the document while the making writes it.
func (m *making) draftOf(unit track.Unit) draftInput {
	start, end := unit.Span()
	in := draftInput{unit: unit}
	for _, l := range m.doc.Lines(unit.Gen) {
		mid := (l.Start + l.End) / 2
		switch {
		case mid < start:
			in.earlier = append(in.earlier, l.Original.Text)
		case mid < end:
			in.ids = append(in.ids, l.ID)
			in.lines = append(in.lines, core.Segment{Start: l.Start, End: l.End, Text: l.Original.Text})
		}
	}
	in.earlier = in.earlier[max(len(in.earlier)-draftContext, 0):]
	return in
}

// draft translates a cell's lines into language, with the lines said before them as context.
func (m *making) draft(ctx context.Context, in draftInput, language string, out chan<- drafted) {
	translated, err := m.p.Translate(WithEarlier(ctx, in.earlier), in.lines, language)
	var partial *UntranslatedError
	if errors.As(err, &partial) && len(translated) == len(in.lines) {
		err = nil
	}
	if err == nil && len(translated) != len(in.lines) {
		err = fmt.Errorf("%d translated lines for %d lines", len(translated), len(in.lines))
	}
	out <- drafted{unit: in.unit, ids: in.ids, lines: translated, err: err}
}

// commitDraft writes a cell's translations, then that its draft is made.
func (m *making) commitDraft(d drafted) {
	var events []track.Event
	for i, s := range d.lines {
		if text := strings.TrimSpace(s.Text); text != "" {
			events = append(events, track.Event{Kind: track.KindRev, Line: d.ids[i], Side: track.Translation, Layer: track.LayerDraft, Text: text, Source: makeSource})
		}
	}
	_ = m.append(append(events, d.unit.Made())...)
}

// progress reports how far the film is made: hearing while cells are left to hear, then translating.
func (m *making) progress(p track.Plan) {
	stage := core.StageTranscribing
	if len(p.Hear) == 0 {
		stage = core.StageTranslating
	}
	done := float64(p.Heard+p.Drafted) / float64(max(2*p.Cells, 1))
	if m.job.Stage != stage {
		m.job.Stage, m.job.Progress = stage, core.Progress{}
	}
	progress := core.Fraction(done)
	progress.Started, progress.From = m.job.Progress.Started, m.job.Progress.From
	if progress.Started.IsZero() {
		progress.Started, progress.From = time.Now(), done
	}
	m.job.Progress = progress
	m.report(*m.job)
}

// project writes the film's lines out beside the video, its transcript and its bilingual subtitles, as they are now, at most every projectEvery unless final, and records what it wrote so a read does not take the files back in as written by something else (ReadTrack). The film shows its bilingual subtitles from the first lines written out.
func (m *making) project(final bool) {
	if !final && time.Since(m.projected) < projectEvery {
		return
	}
	lines := m.doc.Lines(m.generationShown())
	if len(lines) == 0 {
		return
	}
	transcript := []byte(formatSRT(track.Originals(lines)))
	bilingual := []byte(formatSRT(track.Bilingual(lines)))
	if writeFileAtomic(m.transcript, transcript) != nil || writeFileAtomic(m.bilingual, bilingual) != nil {
		return
	}
	m.projected = time.Now()
	_ = m.append(
		track.Event{Kind: track.KindFile, Source: filepath.Base(m.transcript), Key: trackFileHash(transcript, nil)},
		track.Event{Kind: track.KindFile, Source: filepath.Base(m.bilingual), Key: trackFileHash(bilingual, transcript)},
	)
	if m.job.SubtitlePath != m.bilingual {
		m.job.SubtitlePath = m.bilingual
		m.report(*m.job)
	}
}

// generationShown is the generation the film shows, the one being made.
func (m *making) generationShown() string { return m.doc.Shown() }

// runOf is the cells the next run of the speech recognition hears: from the first cell needed, those after it that are needed too, as many as a run hears, or the first alone when the language is not known yet, so it is found before the rest are heard.
func runOf(needed []track.Unit, known bool) []int {
	cells := []int{needed[0].Cell}
	if !known {
		return cells
	}
	for _, u := range needed[1:] {
		if u.Cell != cells[len(cells)-1]+1 || len(cells) == runCells {
			break
		}
		cells = append(cells, u.Cell)
	}
	return cells
}

func containsCell(cells []int, cell int) bool {
	for _, c := range cells {
		if c == cell {
			return true
		}
	}
	return false
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
