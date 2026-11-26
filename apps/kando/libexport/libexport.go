// Package libexport is 导出资料库的字幕: it exports the subtitles of every film of a library that has some, as the player shows them, with the user's corrections and without the lines marked as no speech (core.Correct), beside each video for another player, or into a folder the user picks, laid out as the library is. This file is what it does to each film, which reads no state of the app's; dialog.go is the dialog.
package libexport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"kando/apps/kando/subexport"
	"kando/packages/core"
	"kando/packages/media"
	"kando/packages/track"
)

// Tag is the language code a player such as Plex or Jellyfin reads in a subtitle file's name for target, the language a film is translated into as the settings name it (core.Job.Language): ISO 639-1, as zh for Simplified Chinese, which every player reads; Traditional Chinese is zh-Hant, which recent ones tell apart, so the two never take the same name. "" for a language Kando does not know.
func Tag(target string) string {
	tag := core.TargetTag(target)
	if tag == "zh-Hans" {
		return "zh"
	}
	return tag
}

// Name is the name of the file c exports of the film whose video is at video, translated into target and spoken in spoken (whisper's code), with the extension ext, its dot included: the video's name, then the language its lines are in, the language a player reads them as (Tag) — the target's for a translation or both lines, the spoken one's for the original — as "lesson.zh.srt". A film whose language is not known takes the name the 导出字幕 dialog gives it (subexport.Choice.FileNameExt). The names never take one of those Kando keeps a film's subtitles under (<video>.srt, <video>.kando.srt, <video>.bilingual.srt).
func Name(video string, c subexport.Choice, target, spoken, ext string) string {
	lang := Tag(target)
	if c.Content == core.SubOriginal {
		lang = core.BaseLanguage(spoken)
	}
	if lang == "" || lang == "srt" || lang == "kando" || lang == "bilingual" {
		return c.FileNameExt(video, ext)
	}
	return strings.TrimSuffix(filepath.Base(video), filepath.Ext(video)) + "." + lang + ext
}

// Lines are the lines of the film whose own files are named after base (core.FileBase), whose subtitle file is subtitles, as the player shows them: read through its document (media.ReadTrack), with the user's corrections laid over them.
func Lines(base, subtitles string) ([]core.Corrected, error) {
	edits, err := media.LoadCorrections(media.CorrectionsPath(subtitles))
	if err != nil {
		return nil, err
	}
	doc, err := media.ReadTrack(base, subtitles, media.TranscriptOf(base))
	if err != nil {
		return nil, err
	}
	cues := track.Bilingual(doc.Redoing("", false))
	lines := make([]core.Line, len(cues))
	for i, c := range cues {
		translation, original := core.SplitCue(c.Text)
		lines[i] = core.Line{Start: c.Start, End: c.End, Original: original, Translation: translation}
	}
	return core.Correct(lines, edits), nil
}

// LedgerName is the file that keeps what Kando exported into a folder (Ledger): in the folder of the films' own files beside the videos (media.FilesDir) for an export beside them, and at the top of a folder an export was laid out in.
const LedgerName = ".exports.json"

// Ledger is what Kando exported into a folder, each file by its path from the ledger's folder with the hash of what it wrote: a file there that still holds that is Kando's to write again; any other is the user's, which an export never writes over.
type Ledger map[string]string

// LoadLedger reads the ledger at path; one that is not there, or cannot be read, is empty, so every file already there is taken for the user's.
func LoadLedger(path string) Ledger {
	data, err := os.ReadFile(path)
	if err != nil {
		return Ledger{}
	}
	var l Ledger
	if json.Unmarshal(data, &l) != nil || l == nil {
		return Ledger{}
	}
	return l
}

// Save writes the ledger to path, made with the hidden folder it is in when that is not there, after reading it again, so what another computer exported there meanwhile is kept.
func (l Ledger) Save(path string) error {
	if err := os.Mkdir(filepath.Dir(path), 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	if filepath.Base(filepath.Dir(path)) == media.FilesDir {
		media.HideFilesDir(filepath.Dir(path))
	}
	merged := LoadLedger(path)
	for name, sum := range l {
		merged[name] = sum
	}
	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	return subexport.Write(path, append(data, '\n'))
}

// Outcome is what an export did with a film's file.
type Outcome int

const (
	// Written is a file written, new or Kando's own export made again.
	Written Outcome = iota
	// Same is Kando's own export left as it is, as it already holds what the export holds, so a player does not read it again.
	Same
	// Kept is a file of the user's under the name the export would take, which it never writes over.
	Kept
)

// Write writes data, the export of a film, to path, as l, the ledger that keeps it as key, lets it (Ledger), and keeps what it wrote in l. The folder path is in is made when it is not there, as in a folder an export is laid out in.
func Write(path, key string, data []byte, l Ledger) (Outcome, error) {
	name := filepath.ToSlash(key)
	sum := hash(data)
	if old, err := os.ReadFile(path); err == nil {
		switch {
		case hash(old) == sum:
			l[name] = sum
			return Same, nil
		case l[name] != hash(old):
			return Kept, nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Kept, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Kept, err
	}
	if err := subexport.Write(path, data); err != nil {
		return Kept, err
	}
	l[name] = sum
	return Written, nil
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// OldBeside is what a Kando before this one wrote beside the video at video, under the names only Kando gives a film's files (its bilingual subtitles, the transcript it kept aside, and its hidden files), when the film has its folder of its own files home now, which holds them since: other players may load the bilingual ones over an export. The transcript under the video's own name (<video>.srt) is never among them, as it may be the user's. None for a film whose files are still beside its video.
func OldBeside(video, home string) []string {
	if home == "" {
		return nil
	}
	if info, err := os.Stat(home); err != nil || !info.IsDir() {
		return nil
	}
	// The transcript under the video's own name is the first of the names Kando reads its subtitles by (media.SubtitleNames).
	own := filepath.Clean(media.SubtitleNames(video)[0])
	var out []string
	for _, path := range media.Companions(video) {
		if filepath.Clean(path) != own {
			out = append(out, path)
		}
	}
	return out
}
