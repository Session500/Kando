package media

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"kando/packages/track"
)

// CompanionAt is where the file at path, one of the files named after the video at from (Companions), is named after to instead: the same suffix after to's name, in to's folder, as ".lesson.edits.json" beside lesson.mp4 is ".video.edits.json" beside video.mp4. A path not named after from keeps its name, in to's folder.
func CompanionAt(path, from, to string) string {
	name := filepath.Base(path)
	stem := strings.TrimSuffix(filepath.Base(from), filepath.Ext(from))
	next := strings.TrimSuffix(filepath.Base(to), filepath.Ext(to))
	switch {
	case strings.HasPrefix(name, "."+stem+"."):
		name = "." + next + strings.TrimPrefix(name, "."+stem)
	case strings.HasPrefix(name, stem+"."):
		name = next + strings.TrimPrefix(name, stem)
	}
	return filepath.Join(filepath.Dir(to), name)
}

// kandoBeside is the files Kando wrote beside the video at video that a film named after another path takes with it (AdoptFiles), each with where it goes when the film's files are named after base: every one of Companions but the subtitles of the user's that lie there under the name Kando's transcript would take (<video>.srt beside a <video>.kando.srt Kando kept aside, or one Kando did not write, foreignTranscript) and the sound a link's job keeps until its video comes, which a video picked on disk never has and which is no small file to copy. Kando's transcript is <base>.srt, as no file of the user's lies beside base.
func kandoBeside(video, base string) [][2]string {
	user := ""
	if fileExists(asideTranscriptPath(video)) || foreignTranscript(video) {
		user = filepath.Clean(transcriptPath(video))
	}
	sounds := map[string]bool{}
	for _, p := range soundPaths(video) {
		sounds[filepath.Clean(p)] = true
	}
	var out [][2]string
	for _, p := range Companions(video) {
		clean := filepath.Clean(p)
		if clean == user || sounds[clean] {
			continue
		}
		to := CompanionAt(p, video, base)
		if clean == filepath.Clean(asideTranscriptPath(video)) {
			to = transcriptPath(base)
		}
		out = append(out, [2]string{p, to})
	}
	return out
}

// AdoptFiles copies the files Kando wrote beside the video at video, as a Kando before this one did (kandoBeside), into the folder of base, the path the film's files are named after from now on (core.FileBase), under base's names, and returns where each went, by its path beside the video. It never changes, moves or removes a file beside the video: those are the user's folder's, and another computer that shares it may still read them.
//
// Each file is copied to a hidden name in base's folder first and checked against the one it was copied from, size and hash, before it takes its name, so a copy cut short by a crash or a full disk never stands under a film's name. A file already there under that name is kept as it is and not written over, as one a copy before a crash left; it still counts as where its file went. The folder is made with os.Mkdir, so a library folder that is not there is not made again; it is not made at all when there is nothing to copy. The device's log of the film is told of each subtitle file it took in or wrote out under its old name that holds the same now under its new one (track.KindFile), so it is not taken in again as a file another editor wrote (ReadTrack); one edited outside Kando since is taken in under its new name, as it would have been under its old one.
//
// An error takes back the copies this call made, and nothing beside the video changed: the caller does not name the film's files after base then, and goes on reading and writing them beside the video, so a later try must copy what is there then, not keep a copy of what was there before. undo takes them back too, for a caller that cannot name the film's files after base once they are copied, as one whose library.json cannot be saved: a copy left would hide what is written beside the video until the next try.
func AdoptFiles(video, base, device string) (placed map[string]string, undo func(), err error) {
	files := kandoBeside(video, base)
	if len(files) == 0 {
		return nil, func() {}, nil
	}
	if err := os.Mkdir(filepath.Dir(base), 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, nil, err
	}
	before, err := track.Read(video)
	if err != nil {
		return nil, nil, err
	}
	var made []string
	takeBack := func() {
		for _, path := range made {
			os.Remove(path)
		}
	}
	defer func() {
		if err != nil {
			takeBack()
		}
	}()
	placed = make(map[string]string, len(files))
	for _, f := range files {
		copied, err := copyChecked(f[0], f[1])
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", filepath.Base(f[0]), err)
		}
		if copied {
			made = append(made, f[1])
		}
		placed[f[0]] = f[1]
	}
	if device == "" {
		return placed, takeBack, nil
	}
	after, err := track.Read(base)
	if err != nil {
		return nil, nil, err
	}
	var events []track.Event
	transcript := TranscriptOf(base)
	for _, subtitles := range []string{BilingualPath(base), transcript} {
		from := ""
		for old, next := range placed {
			if filepath.Clean(next) == filepath.Clean(subtitles) {
				from = old
			}
		}
		was, ok := before.File(filepath.Base(from))
		if from == "" || !ok {
			continue
		}
		pairWith := transcript
		if subtitles == transcript {
			pairWith = ""
		}
		if _, file, err := trackFileLines(after, subtitles, pairWith); err == nil && file.Kind != "" && file.Key == was.Key {
			events = append(events, file)
		}
	}
	if len(events) > 0 {
		if _, err := track.Append(base, device, events...); err != nil {
			return nil, nil, err
		}
	}
	return placed, takeBack, nil
}

// copyChecked copies the file at from to to through a hidden file beside to, which takes to's name only once it holds what from holds, size and hash, readable by every account as a file Kando writes is (writeFileAtomic), so a library on a disk shared with another computer reads it there; a file already at to is left as it is. It reports whether it copied one.
func copyChecked(from, to string) (bool, error) {
	if _, err := os.Lstat(to); err == nil {
		return false, nil
	}
	data, err := os.ReadFile(from)
	if err != nil {
		return false, err
	}
	temp, err := os.CreateTemp(filepath.Dir(to), "."+filepath.Base(to)+".*.adopting")
	if err != nil {
		return false, err
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return false, err
	}
	if err := temp.Chmod(0o644); err != nil {
		temp.Close()
		return false, err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return false, err
	}
	if err := temp.Close(); err != nil {
		return false, err
	}
	if err := sameFile(temp.Name(), data); err != nil {
		return false, err
	}
	if _, err := os.Lstat(to); err == nil {
		return false, nil
	}
	return true, os.Rename(temp.Name(), to)
}

// errCopyDiffers is a copy that does not hold what it was copied from.
var errCopyDiffers = errors.New("the copy does not match the file")

// sameFile checks that the file at path holds data, size and hash.
func sameFile(path string, data []byte) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return err
	}
	want := sha256.Sum256(data)
	if n != int64(len(data)) || !bytes.Equal(h.Sum(nil), want[:]) {
		return errCopyDiffers
	}
	return nil
}
