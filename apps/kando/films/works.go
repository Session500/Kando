package films

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"kando/packages/core"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// Works: the library shows the episodes of a show, the lessons of a course or the videos of a playlist as one card, a work, when it is sure they are one. A work is a run of films one after another in one of Next's sequences, so the films folded into a card are in the order they play on, and nothing about what plays next changes for it: a sequence may hold more than its works, as play goes on from a course's last lesson into the folder beside it. A work is only made where the films' list or names leave no doubt; a run that might be films that only sit side by side stays a card per film, as a wrong card stays on the page while a wrong next film plays once.
//
//   - A list's films are one work when the list is one (core.Job.ListTitle): a playlist, or a subscription the user said is a series. A channel's videos are not.
//   - A local sequence is cut where the folder its films' names are in changes, a season's or a part's folder (core.SectionFolder) counting as its parent, so Show/Season 1 and Show/Season 2 are one work. A run is a work when it has two films or more, no two of which number the same episode in the same folder, and its names name a show; or, for names that number an episode without a show (01.mkv, EP05), when it is not at the top of the library or of a media source, its numbers are not scattered, and numbers alone are written alike (01, 02 … 12, not 1, 10, 99).

// Work is a run of films one card shows. Key tells it from the others of the library: "list:" and the list's key, or "show:", the folder it is in and the show's key. Folder is that folder, "" for a list's; Show the show's key, "" for names that number episodes alone. Name is what its card is called, and Films its films' keys in the order they play.
type Work struct {
	Key, Name    string
	Folder, Show string
	List         string
	Films        []string
}

// WorkIndex is the works of a library by the key of each of their films; a film in none is not in it.
type WorkIndex map[string]*Work

// Of is the work the film keyed key is in.
func (w WorkIndex) Of(key string) (*Work, bool) {
	work, ok := w[key]
	return work, ok
}

// Works is the works of all, the films of the library at root, as tag, the interface's language, orders their names. It reads each film's name once and orders each sequence once, so a large library's are had in one pass.
func Works(all []Film, root, tag string) WorkIndex {
	col := collate.New(language.Make(tag), collate.Numeric)
	lists := map[string][]Film{}
	var listOrder []string
	shows := map[string][]Film{}
	var showOrder []string
	// numbered is the local films each folder holds whose names number them, by the show's key, which tells a folder that holds one show from one that holds several.
	numbered := map[string]map[string]bool{}
	for _, f := range all {
		if f.List != "" {
			if _, ok := lists[f.List]; !ok {
				listOrder = append(listOrder, f.List)
			}
			lists[f.List] = append(lists[f.List], f)
			continue
		}
		key, ok := showOf(f)
		if !ok {
			continue
		}
		folder := workFolder(f.VideoPath)
		if numbered[folder] == nil {
			numbered[folder] = map[string]bool{}
		}
		numbered[folder][key] = true
		bucket := filepath.Dir(filepath.Dir(f.VideoPath)) + "\x00" + key
		if _, ok := shows[bucket]; !ok {
			showOrder = append(showOrder, bucket)
		}
		shows[bucket] = append(shows[bucket], f)
	}
	out := WorkIndex{}
	add := func(w *Work) {
		for _, key := range w.Films {
			out[key] = w
		}
	}
	for _, list := range listOrder {
		if w, ok := listWork(list, lists[list]); ok {
			add(w)
		}
	}
	for _, bucket := range showOrder {
		seq := shows[bucket]
		sortLocal(seq, col)
		for _, run := range folderRuns(seq) {
			if w, ok := localWork(run, root, numbered); ok {
				add(w)
			}
		}
	}
	return out
}

// listWork is the work of a list's films, in the list's order, when the list is one (core.Job.ListTitle): it is named as most of its films name it, as one added after the list was renamed may name it otherwise.
func listWork(list string, films []Film) (*Work, bool) {
	if len(films) < 2 {
		return nil, false
	}
	counts := map[string]int{}
	title := ""
	for _, f := range films {
		if f.ListTitle == "" {
			continue
		}
		counts[f.ListTitle]++
		if c := counts[f.ListTitle]; c > counts[title] || c == counts[title] && f.ListTitle < title {
			title = f.ListTitle
		}
	}
	if title == "" {
		return nil, false
	}
	seq := slices.Clone(films)
	slices.SortStableFunc(seq, func(a, b Film) int { return listPlace(a) - listPlace(b) })
	name := core.WithoutTags(title)
	if name == "" {
		name = title
	}
	return &Work{Key: "list:" + list, Name: name, List: list, Films: Keys(seq)}, true
}

// folderRuns cuts a local sequence where the folder its films are in changes (workFolder).
func folderRuns(seq []Film) [][]Film {
	var runs [][]Film
	start := 0
	for i := 1; i <= len(seq); i++ {
		if i == len(seq) || workFolder(seq[i].VideoPath) != workFolder(seq[start].VideoPath) {
			runs = append(runs, seq[start:i])
			start = i
		}
	}
	return runs
}

// workFolder is the folder a local film's work is in: the one its name is in (nameFolder), or that one's parent for a season's or a part's folder (core.SectionFolder).
func workFolder(path string) string {
	folder := nameFolder(path)
	if core.SectionFolder(filepath.Base(folder)) {
		return filepath.Dir(folder)
	}
	return folder
}

// localWork is the work of run, films of a local sequence in one folder (folderRuns), when it is sure they are one: see Works. numbered is the shows each folder's numbered films are of, which names the work.
func localWork(run []Film, root string, numbered map[string]map[string]bool) (*Work, bool) {
	if len(run) < 2 {
		return nil, false
	}
	type place struct {
		folder          string
		season, episode int
	}
	seen := map[place]bool{}
	least, most := -1, -1
	marked, padded, width := false, false, -1
	alike := true
	for _, f := range run {
		name := seqName(f)
		season, episode, _ := core.EpisodeNumber(name)
		at := place{nameFolder(f.VideoPath), season, episode}
		if seen[at] {
			return nil, false
		}
		seen[at] = true
		if least < 0 || episode < least {
			least = episode
		}
		most = max(most, episode)
		digits, explicit, _ := core.EpisodeMark(name)
		marked = marked || explicit
		if !explicit {
			padded = padded || len(digits) > 1 && digits[0] == '0'
			if width >= 0 && width != len(digits) {
				alike = false
			}
			width = len(digits)
		}
	}
	first := run[0]
	key, _ := showOf(first)
	folder := workFolder(first.VideoPath)
	top := workTop(folder, root, first)
	show, _, _ := core.EpisodeOf(seqName(first))
	if key == "" {
		// Names that number episodes alone say nothing of what they are of but their numbers: at the top of a library or a source, or numbered 1, 10 and 99, they are films that sit side by side.
		if top || most-least+1 > 2*len(run) {
			return nil, false
		}
		if !marked && !(padded && alike) {
			return nil, false
		}
	}
	name := show
	if !top && (key == "" || len(numbered[folder]) == 1) {
		name = folderName(folder)
	}
	if name == "" {
		name = show
	}
	if name == "" {
		return nil, false
	}
	return &Work{Key: "show:" + folder + "\x00" + key, Name: name, Folder: folder, Show: key, Films: Keys(run)}, true
}

// workTop reports whether folder, where a work of the library at root is, is the top of the library or of the media source its film f was indexed from, or of a disk: a folder whose name is not the work's.
func workTop(folder, root string, f Film) bool {
	folder = filepath.Clean(folder)
	if folder == filepath.Dir(folder) {
		return true
	}
	if root == "" {
		return false
	}
	root = filepath.Clean(root)
	if folder == root {
		return true
	}
	// A media source's films are said to be at <library>/<source's name>/<their path on it> (Film.Linked).
	if f.Linked != "" {
		if rel, err := filepath.Rel(root, folder); err == nil && filepath.IsLocal(rel) && !strings.ContainsRune(rel, filepath.Separator) {
			return true
		}
	}
	return false
}

// lessonFolder is how a course names its sections' folders, a number written with a leading zero first: 01 Getting Started.
var lessonFolder = regexp.MustCompile(`^0\d+[\s._-]`)

// folderName is the name a work takes from its folder, its tags left out: "Course · 01 Getting Started" for a section of a course named as courses name them (lessonFolder), which says nothing of the course without its parent's name.
func folderName(folder string) string {
	name := core.WithoutTags(filepath.Base(folder))
	if lessonFolder.MatchString(name) {
		if parent := core.WithoutTags(filepath.Base(filepath.Dir(folder))); parent != "" && parent != "." && parent != string(filepath.Separator) {
			return parent + " · " + name
		}
	}
	return name
}
