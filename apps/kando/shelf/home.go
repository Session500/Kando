package shelf

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"kando/packages/core"
)

// HomeFor is the folder in the library bundle at root that Kando keeps the own files of the film whose video, kept outside the library, is at video in (core.Job.Home): "<the video's name> [<id>]", id the first 8 characters of key, the job's ID, or for a video added without a job the hash of its path (AddedKey), so the folder of a film is the same whenever it is given and a try cut short by a crash finds it again. A video named video.<ext> takes its folder's name, as the shelf names it. A video named by its episode's number alone, as 01.mkv or 第3话.mp4 (core.BareEpisode), takes its folder's name before it, and its folder's parent's before that for a season's or a part's folder (core.SectionFolder), as "Show Season 2 01 [id]": the folder is still named by the video's own path alone, whatever else the library holds, and it is found by its [id] whatever its name (media.RenamedFolder).
func HomeFor(root, video, key string) string {
	name := filepath.Base(video)
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	dir := filepath.Dir(video)
	if stem == "video" {
		stem = filepath.Base(dir)
		dir = filepath.Dir(dir)
	}
	stem = homeName(stem, homeStemMax)
	// What the video's name leaves out is cut short before the video's own name is.
	if core.BareEpisode(stem) {
		if within := homeStemMax - len(stem) - 1; within > 0 {
			if before := homeName(homeFolders(dir), within); before != "" {
				stem = before + " " + stem
			}
		}
	}
	if stem == "" {
		stem = "video"
	}
	id := key
	if len(id) > 8 {
		id = id[:8]
	}
	return filepath.Join(root, stem+" ["+id+"]")
}

// homeFolders is what the folder dir of a video named by its episode's number alone says the video is of: its name without its tags, after its parent's for a season's or a part's folder; "" for a folder that names nothing, as the top of a disk.
func homeFolders(dir string) string {
	folder := func(dir string) string {
		name := filepath.Base(dir)
		if name == "." || name == string(filepath.Separator) || name == filepath.VolumeName(dir) {
			return ""
		}
		return core.WithoutTags(name)
	}
	name := folder(dir)
	if name != "" && core.SectionFolder(name) {
		if parent := folder(filepath.Dir(dir)); parent != "" {
			name = parent + " " + name
		}
	}
	return name
}

// homeName is name made one a folder on a Windows, exFAT or SMB disk can have, at most max bytes; "" for a name of nothing but dots and spaces.
func homeName(name string, max int) string {
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name)
	for len(name) > max {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	// A name of nothing but dots, or one Windows would trim, still makes a folder of its own.
	return strings.TrimRight(strings.TrimSpace(name), ". ")
}

// homeStemMax is the most bytes of a video's name its folder takes (HomeFor): a folder's name is at most 255 bytes, and the files in it are named after video.<ext>, so the path of each stays short of what Windows takes.
const homeStemMax = 120

// AddedKey is the key HomeFor names the folder of a video added without a job by: the hash of its cleaned path, so a job made for the video later takes the same folder (Added.Home).
func AddedKey(video string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(video)))
	return hex.EncodeToString(sum[:])
}
