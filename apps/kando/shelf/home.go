package shelf

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
)

// HomeFor is the folder in the library bundle at root that Kando keeps the own files of the film whose video, kept outside the library, is at video in (core.Job.Home): "<the video's name> [<id>]", id the first 8 characters of key, the job's ID, or for a video added without a job the hash of its path (AddedKey), so the folder of a film is the same whenever it is given and a try cut short by a crash finds it again. A video named video.<ext> takes its folder's name, as the shelf names it.
func HomeFor(root, video, key string) string {
	name := filepath.Base(video)
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if stem == "video" {
		stem = filepath.Base(filepath.Dir(video))
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

// AddedKey is the key HomeFor names the folder of a video added without a job by: the hash of its cleaned path, so a job made for the video later takes the same folder (Added.Home).
func AddedKey(video string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(video)))
	return hex.EncodeToString(sum[:])
}
