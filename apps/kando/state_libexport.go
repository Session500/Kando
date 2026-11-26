package main

import (
	"path/filepath"
	"strings"
	"time"

	"kando/apps/kando/libexport"
	"kando/apps/kando/locales"
	"kando/apps/kando/subexport"
	"kando/packages/core"
	"kando/packages/media"

	ns "github.com/nsgo-dev/nsgo/core"
)

// libexportUI is 导出资料库的字幕 (libexport): whether its dialog is open, with the modal flag it holds (holdModal); where the export goes, beside each video or into folder; and the export running or done: how many films it did of how many, what it did, and what a Kando before left beside the videos that it offers to move to the Trash.
type libexportUI struct {
	open    bool
	release func()
	beside  bool
	folder  string
	at      int
	total   int
	result  libexport.Result
	done    bool
	old     []string
	oldOf   int
}

func newLibexportUI() libexportUI { return libexportUI{beside: true} }

// libexportKeys is the File menu's 导出资料库的字幕…, which has no shortcut of its own.
func (a *app) libexportKeys(cx *ns.Scope) []shortcut {
	return []shortcut{{Group: groupFile, Label: locales.LibexportMenu(cx), When: func() bool { return a.LibexportFilms() > 0 }, Run: a.OpenLibexport}}
}

// libexportOverlay is the dialog over the window while it is open (overlays).
func (a *app) libexportOverlay(*ns.Scope) ns.Node { return libexport.Dialog{A: a} }

// libexportFilm is what the export reads and writes of one film, read on the engine's goroutine before it runs.
type libexportFilm struct {
	video, base, subtitles, target, spoken string
	delay                                  time.Duration
	// beside is whether its export can go beside its video: one of the user's, on this computer, outside any library bundle, as a bundle's own videos are Kando's.
	beside bool
	// rel is its video's path in the library's folder, which an export into a folder lays it out by.
	rel string
	// home is the folder of its own files, which holds what a Kando before wrote beside its video once it has one (libexport.OldBeside).
	home string
}

// libexportFilms are the open library's films that have subtitles to export: done, their subtitles named, their video known.
func (a *app) libexportFilms() []libexportFilm {
	root := a.snap().LibraryDir
	var out []libexportFilm
	for _, item := range a.films() {
		if item.SubtitlePath == "" || item.VideoPath == "" || item.Missing || (item.Stage != "" && !item.Stage.Done()) {
			continue
		}
		rel, err := filepath.Rel(root, item.VideoPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			rel = filepath.Base(item.VideoPath)
		}
		spoken := item.Spoken
		if item.SpeechLanguage != "" {
			spoken = item.SpeechLanguage
		}
		out = append(out, libexportFilm{
			video: item.VideoPath, base: item.FileBase(), subtitles: item.SubtitlePath, target: item.Language, spoken: spoken,
			delay:  a.player.delays[item.VideoPath],
			beside: item.Stored == "" && item.Linked == "" && !item.SubtitlesOnly && !inBundle(item.VideoPath),
			rel:    rel, home: item.Home,
		})
	}
	return out
}

func (a *app) LibexportOpen() bool { return a.libexport.open }

// OpenLibexport opens the dialog afresh, beside the videos when any film can go there.
func (a *app) OpenLibexport() {
	if a.libexport.open || a.LibexportFilms() == 0 {
		return
	}
	folder := a.libexport.folder
	a.libexport = newLibexportUI()
	a.libexport.folder = folder
	a.libexport.beside = a.LibexportBesideOK()
	a.libexport.open, a.libexport.release = true, a.holdModal()
}

// CloseLibexport closes the dialog and gives back the window's modal flag; an export running goes on to its end, which the message bar then says.
func (a *app) CloseLibexport() {
	if a.libexport.release != nil {
		a.libexport.release()
	}
	a.libexport.open, a.libexport.release = false, nil
}

// LibexportChoice is the 导出字幕 dialog's choice, SubRip in place of a plugin's format, which exports one film at a time.
func (a *app) LibexportChoice() subexport.Choice {
	c := a.export.choice
	if strings.HasPrefix(c.Format, subexport.PluginPrefix) {
		c.Format = core.FormatSRT
	}
	return c
}

// SetLibexportChoice keeps c as the 导出字幕 dialog's choice too, which both dialogs share.
func (a *app) SetLibexportChoice(c subexport.Choice) { a.SetSubexportChoice(c) }

func (a *app) LibexportFilms() int { return len(a.libexportFilms()) }

func (a *app) LibexportBeside() bool { return a.libexport.beside && a.LibexportBesideOK() }

func (a *app) SetLibexportBeside(on bool) { a.libexport.beside = on }

// LibexportBesideOK reports whether any film of the library can be exported beside its video (libexportFilm.beside).
func (a *app) LibexportBesideOK() bool {
	for _, f := range a.libexportFilms() {
		if f.beside {
			return true
		}
	}
	return false
}

func (a *app) LibexportFolder() string { return a.libexport.folder }

// LibexportPickFolder asks for the folder the export goes into.
func (a *app) LibexportPickFolder() {
	m := a.messages()
	a.pickFolder(ns.PickDirectoriesOptions{Title: m.LibexportPickTitle(), Button: m.LibexportPickButton(), Start: a.libexport.folder}, func(dir string) {
		a.libexport.folder = dir
	})
}

func (a *app) LibexportProgress() libexport.Progress {
	return libexport.Progress{Done: a.libexport.at, Total: a.libexport.total}
}

func (a *app) LibexportDone() bool { return a.libexport.done }

func (a *app) LibexportResult() libexport.Result { return a.libexport.result }

func (a *app) LibexportOld() int { return a.libexport.oldOf }

// LibexportStart exports every film with subtitles off the engine's goroutine, as the dialog chose, telling the dialog how far it got after each; once done, the dialog says what it did and the message bar too when the dialog was closed meanwhile.
func (a *app) LibexportStart() {
	if a.libexport.total > 0 || a.update == nil {
		return
	}
	list := a.libexportFilms()
	beside, folder := a.LibexportBeside(), a.libexport.folder
	if len(list) == 0 || (!beside && folder == "") {
		return
	}
	choice := a.LibexportChoice()
	placeholder := a.messages().SubexportPlaceholder()
	a.libexport.at, a.libexport.total = 0, len(list)
	update := a.update
	go func() {
		result, old, oldOf := libexportRun(list, choice, placeholder, beside, folder, func(at int) {
			update(func() { a.libexport.at = at })
		})
		update(func() {
			a.libexport.result, a.libexport.done = result, true
			a.libexport.old, a.libexport.oldOf = old, oldOf
			if !a.libexport.open {
				a.tell(libexportResultText(a.messages(), result))
			}
		})
	}()
}

// libexportRun exports list as choice says, beside each video or into folder laid out as the library is, and returns what it did, with what a Kando before left beside the videos of the films exported beside them (libexport.OldBeside) and how many films that is. It runs off the engine's goroutine, reading no state of the app's.
func libexportRun(list []libexportFilm, choice subexport.Choice, placeholder string, beside bool, folder string, progress func(int)) (result libexport.Result, old []string, oldOf int) {
	ledgers := map[string]libexport.Ledger{}
	ledger := func(path string) libexport.Ledger {
		if l, ok := ledgers[path]; ok {
			return l
		}
		l := libexport.LoadLedger(path)
		ledgers[path] = l
		return l
	}
	ext := core.Export{Format: choice.Format}.Ext()
	for i, f := range list {
		progress(i + 1)
		if beside && !f.beside {
			result.Skipped++
			continue
		}
		lines, err := libexport.Lines(f.base, f.subtitles)
		if err != nil {
			result.Failed++
			continue
		}
		data := choice.Export(f.delay, placeholder).Write(lines)
		name := libexport.Name(f.video, choice, f.target, f.spoken, ext)
		var path, ledgerPath, key string
		if beside {
			dir := filepath.Dir(f.video)
			path, ledgerPath, key = filepath.Join(dir, name), filepath.Join(dir, media.FilesDir, libexport.LedgerName), name
		} else {
			rel := filepath.Join(filepath.Dir(f.rel), name)
			path, ledgerPath, key = filepath.Join(folder, rel), filepath.Join(folder, libexport.LedgerName), rel
		}
		switch outcome, err := libexport.Write(path, key, data, ledger(ledgerPath)); {
		case err != nil:
			result.Failed++
		case outcome == libexport.Written:
			result.Written++
		case outcome == libexport.Same:
			result.Same++
		default:
			result.Kept++
		}
		if beside {
			if files := libexport.OldBeside(f.video, f.home); len(files) > 0 {
				old = append(old, files...)
				oldOf++
			}
		}
	}
	for path, l := range ledgers {
		if len(l) > 0 {
			if err := l.Save(path); err != nil {
				result.Failed++
			}
		}
	}
	return result, old, oldOf
}

// libexportResultText is what an export did, as result says, in the language of m, for the message bar.
func libexportResultText(m locales.Messages, result libexport.Result) string {
	parts := []string{m.LibexportWritten(result.Written)}
	if result.Same > 0 {
		parts = append(parts, m.LibexportSame(result.Same))
	}
	if result.Kept > 0 {
		parts = append(parts, m.LibexportKept(result.Kept))
	}
	if result.Skipped > 0 {
		parts = append(parts, m.LibexportSkipped(result.Skipped))
	}
	if result.Failed > 0 {
		parts = append(parts, m.LibexportFailed(result.Failed))
	}
	return strings.Join(parts, " · ")
}

// LibexportTrashOld moves what a Kando before left beside the videos to the Trash, one file after another, and says so in the message bar; the dialog no longer offers it.
func (a *app) LibexportTrashOld() {
	files := a.libexport.old
	a.libexport.old, a.libexport.oldOf = nil, 0
	if len(files) == 0 {
		return
	}
	failed := false
	var next func(i int)
	next = func(i int) {
		if i == len(files) {
			if failed {
				a.warn(a.messages().LibexportTrashFailed(), "")
			} else {
				a.tell(a.messages().LibexportTrashed())
			}
			a.refreshLibrary()
			return
		}
		a.libraryTrash([]string{files[i]}, func(err error) {
			failed = failed || err != nil
			next(i + 1)
		})
	}
	next(0)
}
