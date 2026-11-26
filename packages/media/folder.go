package media

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"kando/packages/core"
)

// FilesDir is the hidden folder beside the videos of a folder of the user's opened as a library, in each folder that holds one, that the films' own files are kept in, one folder of each film's (FolderHome): Kando writes nothing beside a video of the user's. Its name starts with a dot, so the library, Plex and Jellyfin skip it, and it does not end in LibraryExt, so it is never taken for a library bundle.
const FilesDir = ".kando-files"

// folderNameMax is the most bytes of a name a film's folder takes (FolderName): a folder's name is at most 255 bytes, and the files in it are named after video.<ext>, so the path of each stays short of what Windows takes.
const folderNameMax = 120

// FolderName is name made the name of a folder every disk takes: a character a Windows, exFAT or SMB disk refuses is replaced, it is cut to folderNameMax bytes where a character ends, and the dots and spaces Windows would trim from its end are taken off. A name that is then empty, or that starts with a dot, which would hide it, or name a folder's own entries, starts with an underscore.
func FolderName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name)
	for len(name) > folderNameMax {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	name = strings.TrimRight(strings.TrimSpace(name), ". ")
	if name == "" || strings.HasPrefix(name, ".") {
		name = "_" + name
	}
	return name
}

// FolderHome is the folder the own files of the film whose video is at video are kept in, in a folder of the user's opened as a library (core.Job.Home): FilesDir beside the video, and in it a folder named after the video's name without its extension, so the film's files go with it as its folder is renamed in Finder, and as its file is made again in another format.
func FolderHome(video string) string {
	name := filepath.Base(video)
	return filepath.Join(filepath.Dir(video), FilesDir, FolderName(strings.TrimSuffix(name, filepath.Ext(name))))
}

// FolderHomeFor is FolderHome of the video at video when library, the folder of the library it belongs to, is a folder of the user's rather than a library bundle (LibraryExt) and holds it, outside any bundle in it: "" otherwise, for a video a bundle holds, whose files are beside it, and for one kept outside its library, whose folder is the library's own (core.Job.Home).
func FolderHomeFor(library, video string) string {
	if library == "" || video == "" || isBundle(library) {
		return ""
	}
	rel, err := filepath.Rel(filepath.Clean(library), filepath.Clean(video))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return ""
	}
	for dir := filepath.Dir(rel); dir != "." && dir != string(filepath.Separator); dir = filepath.Dir(dir) {
		if isBundle(dir) {
			return ""
		}
	}
	return FolderHome(video)
}

// isBundle reports whether the folder at dir is a library bundle of Kando's own, by its extension.
func isBundle(dir string) bool {
	return strings.EqualFold(filepath.Ext(filepath.Clean(dir)), LibraryExt)
}

// AdoptInto gives the film of the video at video the folder home of its own files (FolderHome), copying into it what a Kando before this one wrote beside the video (AdoptFiles), and reports whether it made it: one that is there already was given before, on this computer or another sharing the folder, and is left as it is, so what was taken out of it does not come back. The files are copied into a hidden folder beside home first, which takes home's name only once all of them are in, so a folder of a film is there only once it is whole, and a copy cut short leaves nothing a later try takes for done. The size of the video is kept in it (FolderVideoChanged). placed is where each file went, by its path beside the video. The folder FilesDir it is in is made when it is not there, hidden (hideFolder); the folder holding the video is never made.
func AdoptInto(video, home, device string) (placed map[string]string, made bool, err error) {
	if _, err := os.Stat(home); err == nil {
		return nil, false, nil
	}
	files := filepath.Dir(home)
	if err := os.Mkdir(files, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, false, err
	}
	hideFolder(files)
	staging, err := os.MkdirTemp(files, "."+filepath.Base(home)+".adopting-")
	if err != nil {
		return nil, false, err
	}
	// Whatever is left of the staging folder goes: renamed to home, nothing is.
	defer os.RemoveAll(staging)
	if err := os.Chmod(staging, 0o755); err != nil {
		return nil, false, err
	}
	staged, _, err := AdoptFiles(video, core.FileBase(video, staging), device)
	if err != nil {
		return nil, false, err
	}
	if err := keepVideoSize(staging, video); err != nil {
		return nil, false, err
	}
	if err := os.Rename(staging, home); err != nil {
		// Another computer sharing the folder, or a try before this one, gave the film its folder meanwhile: it stands.
		if _, statErr := os.Stat(home); statErr == nil {
			return nil, false, nil
		}
		return nil, false, err
	}
	placed = make(map[string]string, len(staged))
	for from, to := range staged {
		placed[from] = filepath.Join(home, filepath.Base(to))
	}
	return placed, true, nil
}

// HideFilesDir marks the folder of films' files at path (FilesDir) hidden where a name starting with a dot does not hide it (hideFolder), as one a move makes.
func HideFilesDir(path string) { hideFolder(path) }

// folderVideoFile is the hidden file in a film's folder (FolderHome) that keeps what the video it was given for was (folderVideo).
const folderVideoFile = ".video.json"

// folderVideo is what a film's folder keeps of the video it was given for, to tell when the file at the video's path is another one (FolderVideoChanged).
type folderVideo struct {
	Size int64 `json:"size"`
}

// keepVideoSize keeps the size of the video at video in the film's folder home (folderVideo); a video that is not there keeps none.
func keepVideoSize(home, video string) error {
	info, err := os.Stat(video)
	if err != nil {
		return nil
	}
	data, err := json.Marshal(folderVideo{Size: info.Size()})
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(home, folderVideoFile), append(data, '\n'))
}

// FolderVideoChanged reports whether the file at video is not the one its film's folder home was given for: a file of another size, as one the user put in its place in Finder under the same name, another episode or another cut of it, whose film would show the other one's subtitles. A folder that kept no size, and a video that is not there, are not.
func FolderVideoChanged(home, video string) bool {
	data, err := os.ReadFile(filepath.Join(home, folderVideoFile))
	if err != nil {
		return false
	}
	var kept folderVideo
	if json.Unmarshal(data, &kept) != nil || kept.Size == 0 {
		return false
	}
	info, err := os.Stat(video)
	return err == nil && info.Size() != kept.Size
}

// KeepFolderVideo keeps the file at video as the one the film's folder home is for, as the user does who keeps its subtitles once told it changed (FolderVideoChanged).
func KeepFolderVideo(home, video string) error { return keepVideoSize(home, video) }

// DropEmptyFolderHome takes away the folder of a film's files home (FolderHome) once nothing of the film is left in it but what it keeps of its video (folderVideo), as after its files moved with the film to another library, and the hidden folder that held it once that holds none (FilesDir). One that still holds a file of the film's, as one a copy could not remove, stays.
func DropEmptyFolderHome(home string) {
	entries, err := os.ReadDir(home)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.Name() != folderVideoFile {
			return
		}
	}
	os.Remove(filepath.Join(home, folderVideoFile))
	os.Remove(home)
	os.Remove(filepath.Dir(home))
}

// folderHomes is the names of the films' folders in the FilesDir of dir, "" when it has none.
func folderHomes(dir string) map[string]bool {
	entries, err := os.ReadDir(filepath.Join(dir, FilesDir))
	if err != nil {
		return nil
	}
	names := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			names[entry.Name()] = true
		}
	}
	return names
}

// filmFiles is the folder of its own files (FolderHome) and the bilingual subtitles of the video named name in dir, as the library lists it: its folder and the subtitles there once it has one, which a folder of the user's opened as a library gives it (homes, folderHomes), else the subtitles beside it (bilingualBeside), files being the names of the files in dir.
func filmFiles(dir, name string, files, homes map[string]bool) (home, subtitles string) {
	video := filepath.Join(dir, name)
	if homes[filepath.Base(FolderHome(video))] {
		home = FolderHome(video)
		if path := BilingualPath(core.FileBase(video, home)); fileExists(path) {
			subtitles = path
		}
		return home, subtitles
	}
	return "", bilingualBeside(dir, name, files)
}
