package films

import (
	"path/filepath"
	"slices"
	"strings"

	"kando/apps/kando/sidebar"
	"kando/packages/core"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// What plays after a film, as a course, a disc, a season or a playlist goes on: the films of one sequence, in its order.
//
// A film added from a list, a playlist or a channel read by the playlist dialog or a subscription (core.Job.List), is in the sequence of that list, in its order (core.Job.ListIndex): a playlist's own, a channel's from its oldest video. A local film, one played where it was picked or found in the library's folder, is in the sequence of the films whose names number them as episodes of the same show (core.EpisodeOf, core.EpisodeNumber) in its folder and the folders beside it, as a course keeps its lessons in a folder per section: folder by folder, then by number. Any other film, as a link added alone, is in none, so films that merely sit side by side, as clips in a Downloads folder, are not played one after another. A film whose job failed or was stopped is passed over.

// Next is the film that plays after item among all, the open library's films, in tag's order of names, the interface's language; false when there is none.
func Next(all []Film, item Film, tag string) (Film, bool) { return stepFrom(all, item, tag, 1) }

// Previous is the film that plays before item among all, as Next orders them; false when there is none.
func Previous(all []Film, item Film, tag string) (Film, bool) { return stepFrom(all, item, tag, -1) }

func stepFrom(all []Film, item Film, tag string, step int) (Film, bool) {
	seq := sequence(all, item, tag)
	at := slices.IndexFunc(seq, func(f Film) bool { return f.Key == item.Key })
	if at < 0 {
		return Film{}, false
	}
	for i := at + step; i >= 0 && i < len(seq); i += step {
		if f := seq[i]; f.Stage != core.StageFailed {
			return f, true
		}
	}
	return Film{}, false
}

// sequence is the films of all in item's sequence, item among them, in their order; nil for a film in none.
func sequence(all []Film, item Film, tag string) []Film {
	if item.List != "" {
		var seq []Film
		for _, f := range all {
			if f.List == item.List {
				seq = append(seq, f)
			}
		}
		// A film of the list with no place yet, as one added before its list was read, comes after those with one.
		place := func(f Film) int {
			if f.ListIndex <= 0 {
				return int(^uint(0) >> 1)
			}
			return f.ListIndex
		}
		slices.SortStableFunc(seq, func(a, b Film) int { return place(a) - place(b) })
		return seq
	}
	key, ok := showOf(item)
	if !ok {
		return nil
	}
	parent := filepath.Dir(filepath.Dir(item.VideoPath))
	var seq []Film
	for _, f := range all {
		if f.List == "" && f.VideoPath != "" && filepath.Dir(filepath.Dir(f.VideoPath)) == parent {
			if k, ok := showOf(f); ok && k == key {
				seq = append(seq, f)
			}
		}
	}
	col := collate.New(language.Make(tag), collate.Numeric)
	slices.SortStableFunc(seq, func(a, b Film) int {
		if c := byNumber(col, filepath.Base(filepath.Dir(a.VideoPath)), filepath.Base(filepath.Dir(b.VideoPath))); c != 0 {
			return c
		}
		return byNumber(col, seqName(a), seqName(b))
	})
	return seq
}

// showOf is the show a local film's name numbers it as an episode of, its key as core.EpisodeOf gives it, "" for a name that numbers it without a show, as a disc's "01 Opening"; false for a film that is not local or whose name numbers it as nothing.
func showOf(f Film) (string, bool) {
	if f.VideoPath == "" || SourceOf(f) != sidebar.SourceLocal {
		return "", false
	}
	name := seqName(f)
	if _, _, ok := core.EpisodeNumber(name); !ok {
		return "", false
	}
	_, key, _ := core.EpisodeOf(name)
	return key, true
}

// seqName is the name a film's file gives it, without its extension: its folder's for a video named video.<ext> in a folder of its own, as the library keeps one.
func seqName(f Film) string {
	base := filepath.Base(f.VideoPath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if name == "video" {
		return filepath.Base(filepath.Dir(f.VideoPath))
	}
	return name
}

// byNumber orders two names by the season and episode they number, then as col orders them, numbers by value.
func byNumber(col *collate.Collator, a, b string) int {
	as, ae, aok := core.EpisodeNumber(a)
	bs, be, bok := core.EpisodeNumber(b)
	if aok && bok {
		if as != bs {
			return as - bs
		}
		if ae != be {
			return ae - be
		}
	}
	return col.CompareString(a, b)
}
