package films

import (
	"bytes"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"kando/apps/kando/sidebar"
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
	// showsIn is the local films each folder holds whose names number them, by the show's key, which tells a folder that holds one show from one that holds several.
	showsIn := map[string]map[string]bool{}
	// read is what each local film's name says of it, by its video's path, and folders the folder each folder's films' work is in (workFolder), each read once.
	read := map[string]localRead{}
	folders := map[string]string{}
	for _, f := range all {
		if f.List != "" {
			if _, ok := lists[f.List]; !ok {
				listOrder = append(listOrder, f.List)
			}
			lists[f.List] = append(lists[f.List], f)
			continue
		}
		if f.VideoPath == "" || SourceOf(f) != sidebar.SourceLocal {
			continue
		}
		name := seqName(f)
		e, ok := core.ReadEpisode(name)
		if !ok {
			continue
		}
		key := e.Key
		dir := nameFolder(f.VideoPath)
		folder, known := folders[dir]
		if !known {
			folder = workFolderOf(dir)
			folders[dir] = folder
		}
		read[f.VideoPath] = localRead{episode: e, dir: dir, folder: folder}
		if showsIn[folder] == nil {
			showsIn[folder] = map[string]bool{}
		}
		showsIn[folder][key] = true
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
		sortLocal(seq, col, func(f Film) (numbered, bool) {
			r, ok := read[f.VideoPath]
			return numbered{r.episode.Season, r.episode.Number, true}, ok
		})
		for _, run := range folderRuns(seq, read) {
			if w, ok := localWork(run, root, showsIn, read); ok {
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

// localRead is what Works read of a local film: what its name says of it as an episode, the folder its name is in (nameFolder) and the one its work is in (workFolder).
type localRead struct {
	episode     core.Episode
	dir, folder string
}

// folderRuns cuts a local sequence where the folder its films' work is in changes (workFolder), as read has it.
func folderRuns(seq []Film, read map[string]localRead) [][]Film {
	var runs [][]Film
	start := 0
	for i := 1; i <= len(seq); i++ {
		if i == len(seq) || read[seq[i].VideoPath].folder != read[seq[start].VideoPath].folder {
			runs = append(runs, seq[start:i])
			start = i
		}
	}
	return runs
}

// workFolder is the folder a local film's work is in: the one its name is in (nameFolder), or that one's parent for a season's or a part's folder (core.SectionFolder).
func workFolder(path string) string { return workFolderOf(nameFolder(path)) }

// workFolderOf is workFolder of a film whose name is in folder.
func workFolderOf(folder string) string {
	if core.SectionFolder(filepath.Base(folder)) {
		return filepath.Dir(folder)
	}
	return folder
}

// localWork is the work of run, films of a local sequence in one folder (folderRuns), when it is sure they are one: see Works. showsIn is the shows each folder's numbered films are of, which names the work, and read what Works read of each film.
func localWork(run []Film, root string, showsIn map[string]map[string]bool, read map[string]localRead) (*Work, bool) {
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
		r := read[f.VideoPath]
		episode := r.episode.Number
		at := place{r.dir, r.episode.Season, episode}
		if seen[at] {
			return nil, false
		}
		seen[at] = true
		if least < 0 || episode < least {
			least = episode
		}
		most = max(most, episode)
		digits, explicit := r.episode.Digits, r.episode.Explicit
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
	key, show, folder := read[first.VideoPath].episode.Key, read[first.VideoPath].episode.Show, read[first.VideoPath].folder
	top := workTop(folder, root, first)
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
	if !top && (key == "" || len(showsIn[folder]) == 1) {
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

// WorksCache keeps the works Works last gave, so a library's are worked out again only once what they are made of changes: each film's key, video, source, list and place and title in it, and whether it was indexed from a media source, with the library and the language. Its zero value is empty. It is the open library's, on the UI thread; another reader works its own out.
type WorksCache struct {
	print []byte
	works WorkIndex
}

// Get is the works of all, the films of the library at root, in tag's order (Works), those it gave last when none of what they are made of changed.
func (c *WorksCache) Get(all []Film, root, tag string) WorkIndex {
	print := worksPrint(all, root, tag)
	if c.works != nil && bytes.Equal(print, c.print) {
		return c.works
	}
	c.print, c.works = print, Works(all, root, tag)
	return c.works
}

// worksPrint is what the works of all are made of, written out (WorksCache): what Works reads and nothing else, so a job's progress does not count.
func worksPrint(all []Film, root, tag string) []byte {
	out := make([]byte, 0, 64*len(all))
	field := func(s string) {
		out = append(out, s...)
		out = append(out, 0)
	}
	field(root)
	field(tag)
	for _, f := range all {
		field(f.Key)
		field(f.VideoPath)
		field(f.Source)
		field(f.List)
		field(strconv.Itoa(f.ListIndex))
		field(f.ListTitle)
		field(f.Linked)
	}
	return out
}

// MatchesWork reports whether the film's name holds query (MatchesTitle), or the name of the work it is in among works does, so a search for a show finds its episodes.
func MatchesWork(item Film, query string, works WorkIndex) bool {
	if MatchesTitle(item, query) {
		return true
	}
	w, ok := works.Of(item.Key)
	return ok && strings.Contains(strings.ToLower(w.Name), strings.ToLower(strings.TrimSpace(query)))
}
