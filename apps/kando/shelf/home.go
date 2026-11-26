package shelf

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// HomeFor is the folder in the library bundle at root that Kando keeps the own files of the film whose video, kept outside the library, is at video in (core.Job.Home): "<the video's name> [<id>]", id the first 8 characters of key, the job's ID, or for a video added without a job the hash of its path (AddedKey), so the folder of a film is the same whenever it is given and a try cut short by a crash finds it again. A video named video.<ext> takes its folder's name, as the shelf names it.
func HomeFor(root, video, key string) string {
	name := filepath.Base(video)
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if stem == "video" {
		stem = filepath.Base(filepath.Dir(video))
	}
	// A name a folder on a Windows, exFAT or SMB disk cannot have, or one too long for a folder's with its [id], is made one that can be, so every video has a folder wherever its library is.
	stem = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, stem)
	for len(stem) > homeStemMax {
		_, size := utf8.DecodeLastRuneInString(stem)
		stem = stem[:len(stem)-size]
	}
	// A name of nothing but dots, or one Windows would trim, still makes a folder of its own.
	stem = strings.TrimRight(strings.TrimSpace(stem), ". ")
	if stem == "" {
		stem = "video"
	}
	id := key
	if len(id) > 8 {
		id = id[:8]
	}
	return filepath.Join(root, stem+" ["+id+"]")
}

// homeStemMax is the most bytes of a video's name its folder takes (HomeFor): a folder's name is at most 255 bytes, and the files in it are named after video.<ext>, so the path of each stays short of what Windows takes.
const homeStemMax = 120

// AddedKey is the key HomeFor names the folder of a video added without a job by: the hash of its cleaned path, so a job made for the video later takes the same folder (Added.Home).
func AddedKey(video string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(video)))
	return hex.EncodeToString(sum[:])
}
