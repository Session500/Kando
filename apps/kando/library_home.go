package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"kando/apps/kando/films"
	"kando/apps/kando/shelf"
	"kando/packages/core"
	"kando/packages/media"
)

// A video kept outside a library bundle, where the user picked it, has a folder of its own in the bundle that Kando writes the film's files in (core.Job.Home, shelf.Added.Home), so nothing is ever written into the user's folder. A film is given its folder, and the files a Kando before this one wrote beside its video are copied in (libraryAdopt), at the only places that come before anything of it is written: as the player opens it (Open), as the queue starts its job (queueRun) and as a paired phone writes to it (phoneAdopt). Until then it reads its files beside its video, as it always did, and nothing writes them.

// libraryHomeable reports whether a film of the library at root whose video is at video is one whose files Kando keeps in a folder of root (core.Job.Home): root is a library bundle of Kando's own that is there and not locked, and the video is outside it. A folder of the user's opened as a library keeps the films' files beside their videos, as do the films a bundle holds.
func (a *app) libraryHomeable(root, video string) bool {
	if root == "" || video == "" || !shelfBundle(root) || films.InDir(root, video) || a.lockFilesAway(root) {
		return false
	}
	info, err := os.Stat(root)
	return err == nil && info.IsDir()
}

// errLibraryGone is why a film's folder in its library (core.Job.Home) cannot be made: the library's folder is not there, or is locked, and is not made again.
var errLibraryGone = errors.New("the library's folder is not there")

// libraryHomeReady makes the folder home of a film's files (core.Job.Home) when it is not there, as after a job that never completed took it to the Trash, or the user removed it in Finder, under its library's folder root, which must be there and not locked: the library is never made again here.
func (a *app) libraryHomeReady(root, home string) error {
	if _, err := os.Stat(home); err == nil {
		return nil
	}
	if root == "" || a.lockFilesAway(root) {
		return errLibraryGone
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return errLibraryGone
	}
	if err := os.Mkdir(home, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	return nil
}

// libraryJobVideo is the video job, a job of a video picked on disk, processes: its VideoPath, or the file its source names before the job first ran. "" for a link's job, one of a media source and one that keeps no video.
func libraryJobVideo(job core.Job) string {
	path := queueLocalPath(job.Source)
	if path == "" || job.Linked || job.SubtitlesOnly {
		return ""
	}
	if job.VideoPath != "" {
		return job.VideoPath
	}
	return path
}

// libraryAdopt gives the film of job its folder in its library (core.Job.Home) before anything of it is written, when it is a video kept outside a library bundle that has none yet, and saves the shelf with it: the files a Kando before this one wrote beside the video are copied in first (media.AdoptFiles), its subtitles named there from then on (SubtitlePath), and nothing beside the video changes. A film that has its folder has it made again when it is gone (libraryHomeReady). job is changed only once the shelf is saved: an error leaves it as it was, and the caller does not go on to write, as its files would then be beside the video. A film that is not such a video is left as it is, its files beside its video as always.
func (a *app) libraryAdopt(job *core.Job) error {
	root := job.LibraryDir
	if job.Home != "" {
		return a.libraryHomeReady(root, job.Home)
	}
	video := libraryJobVideo(*job)
	if !a.libraryHomeable(root, video) {
		return nil
	}
	next := *job
	next.VideoPath = video
	var placed map[string]string
	// A Kando that knew no Home and ran in between, after a newer one gave the film its folder, kept the subtitles there and dropped the folder from library.json: the folder is the film's again, its files as they are.
	if dir := filepath.Dir(job.SubtitlePath); job.SubtitlePath != "" && filepath.Clean(filepath.Dir(dir)) == filepath.Clean(root) {
		next.Home = dir
	} else {
		next.Home = shelf.HomeFor(root, video, job.ID)
		var err error
		if placed, err = a.libraryAdoptFiles(root, video, next.Home); err != nil {
			return err
		}
		next.SubtitlePath = libraryAdoptedPath(job.SubtitlePath, video, next.FileBase(), placed)
	}
	was, _ := a.library.Find(job.ID)
	a.library.Upsert(next)
	if err := a.library.Save(); err != nil {
		if was.ID != "" {
			a.library.Upsert(was)
		}
		return err
	}
	*job = next
	a.libraryAdoptedTell(placed, a.libraryJobName(next))
	return nil
}

// libraryAdoptAdded gives the video at video, added to the library at root without a job (shelf.Library.Added), its folder there as libraryAdopt gives a job's film one, and returns it: "" for a video whose files stay beside it, which is one of no library bundle or not added. The shelf is saved with it, and the open library's list shows it.
func (a *app) libraryAdoptAdded(root, video string) (string, error) {
	added, ok := a.library.AddedIn(root)[video]
	if !ok || !a.libraryHomeable(root, video) {
		return "", nil
	}
	if added.Home != "" {
		return added.Home, a.libraryHomeReady(root, added.Home)
	}
	home := shelf.HomeFor(root, video, shelf.AddedKey(video))
	placed, err := a.libraryAdoptFiles(root, video, home)
	if err != nil {
		return "", err
	}
	a.library.SetAddedHome(root, video, home)
	if err := a.library.Save(); err != nil {
		a.library.SetAddedHome(root, video, "")
		return "", err
	}
	base := core.FileBase(video, home)
	subtitles := ""
	if fileExists(media.BilingualPath(base)) {
		subtitles = media.BilingualPath(base)
	}
	a.edit(func(s *session) {
		for i := range s.Videos {
			if s.Videos[i].Path == video {
				s.Videos[i].Home, s.Videos[i].Subtitles = home, subtitles
			}
		}
	})
	if item, ok := films.Find(a.films(), video); ok {
		a.libraryAdoptedTell(placed, films.Name(item))
	}
	return home, nil
}

// libraryAdoptFiles makes home, the folder of the film's files of the video at video in the library at root, and copies the files Kando wrote beside the video into it (media.AdoptFiles).
func (a *app) libraryAdoptFiles(root, video, home string) (map[string]string, error) {
	if err := a.libraryHomeReady(root, home); err != nil {
		return nil, err
	}
	return media.AdoptFiles(video, core.FileBase(video, home), a.savedDevice())
}

// libraryAdoptedPath is where subtitles, the film's subtitle file beside the video at video, are once its files are named after base: where it was copied to (placed), or the name it would have there when it was not there to copy.
func libraryAdoptedPath(subtitles, video, base string, placed map[string]string) string {
	if subtitles == "" {
		return ""
	}
	if next, ok := placed[subtitles]; ok {
		return next
	}
	if filepath.Clean(filepath.Dir(subtitles)) == filepath.Clean(filepath.Dir(video)) {
		return media.CompanionAt(subtitles, video, base)
	}
	return subtitles
}

// libraryAdoptedTell says in the message bar, once for the film named name, that its subtitles are kept in the library from now on, when any of its files were copied there (placed): those left beside its video are not written again, and 导出字幕 writes one there for another player.
func (a *app) libraryAdoptedTell(placed map[string]string, name string) {
	if len(placed) == 0 {
		return
	}
	a.tell(a.messages().LibraryAdopted(name))
}

// libraryAdoptFilm gives item, the film the player opens, its folder in the library before anything of it is written (libraryAdopt, libraryAdoptAdded), and returns it as it is then. One that could not be given it opens as it is, the message bar saying why.
func (a *app) libraryAdoptFilm(item films.Film) films.Film {
	root := a.snap().LibraryDir
	switch {
	case item.JobID != "":
		job, ok := a.library.Find(item.JobID)
		if !ok {
			return item
		}
		if err := a.libraryAdopt(&job); err != nil {
			a.warn(a.messages().LibraryAdoptFailed(films.Name(item), err.Error()), "")
			return item
		}
		item.Home, item.SubtitlePath = job.Home, job.SubtitlePath
		if item.VideoPath == "" {
			item.VideoPath = job.VideoPath
		}
	case !item.Added.IsZero():
		home, err := a.libraryAdoptAdded(root, item.VideoPath)
		if err != nil {
			a.warn(a.messages().LibraryAdoptFailed(films.Name(item), err.Error()), "")
			return item
		}
		if home != "" && item.Home == "" {
			if refreshed, ok := films.Find(a.films(), item.Key); ok {
				return refreshed
			}
			item.Home = home
		}
	}
	return item
}

// phoneAdopt gives the film of the library at dir a paired phone writes to, whose ID in the protocol is id (phoneFilmID) and whose video is at video, its folder in the library before the phone's changes are written (libraryAdopt, libraryAdoptAdded), and returns what its files are named after then (core.FileBase). A film whose files stay beside its video is named after its video. An error is the phone's call's: what it put is not written.
func (a *app) phoneAdopt(dir, id, video string) (string, error) {
	if job, ok := a.library.Find(id); ok && id != "" {
		if err := a.libraryAdopt(&job); err != nil {
			return "", err
		}
		if job.VideoPath == "" {
			return video, nil
		}
		return job.FileBase(), nil
	}
	home, err := a.libraryAdoptAdded(dir, video)
	if err != nil {
		return "", err
	}
	return core.FileBase(video, home), nil
}

// libraryHomeBases are what the films whose videos are kept outside their library have their files named after there (core.FileBase), video.<ext> in each folder of theirs (core.Job.Home, shelf.Added.Home): no video is at them, and like a video moved to a storage location they belong where they are, so the library does not take the folder for one of its own to gather a video into (media.GatherVideos). They are not films of the folder to list (awayPaths): each is listed by its job or its added video.
func (a *app) libraryHomeBases() []string {
	var out []string
	for _, job := range a.library.Jobs {
		if job.Home != "" {
			out = append(out, job.FileBase())
		}
	}
	for _, videos := range a.library.Added {
		for video, added := range videos {
			if added.Home != "" {
				out = append(out, core.FileBase(video, added.Home))
			}
		}
	}
	return out
}

// retargetHomes follows the folders of the open library's films' files that were renamed in Finder (core.Job.Home, shelf.Added.Home): a folder that is gone is looked for by the [id] in its name (media.RenamedFolder), and the film's subtitles go with it. One not found keeps its folder's path, its film listed as before with its subtitles missing, and is never dropped. It reports whether library.json changed.
func (a *app) retargetHomes() bool {
	root := a.snap().LibraryDir
	if root == "" || !shelfBundle(root) {
		return false
	}
	changed := false
	for i, job := range a.library.Jobs {
		if job.Home == "" || !films.InDir(root, job.Home) {
			continue
		}
		next, ok := media.RenamedFolder(root, job.Home)
		if !ok {
			continue
		}
		if sub, ok := replacePathPrefix(job.Home, next, job.SubtitlePath); ok {
			a.library.Jobs[i].SubtitlePath = sub
		}
		a.library.Jobs[i].Home = next
		changed = true
	}
	for video, added := range a.library.AddedIn(root) {
		if added.Home == "" {
			continue
		}
		if next, ok := media.RenamedFolder(root, added.Home); ok {
			a.library.SetAddedHome(root, video, next)
			changed = true
		}
	}
	return changed
}

// libraryHomeUnmade moves the folder of the files of job's film in its library (core.Job.Home) to the Trash as the job leaves the shelf, when the job never completed (core.Job.CompletedAt) and so made none of the film's subtitles but those it left half made, as one cancelled or failed the first time it made them: a video added without a job, whose film comes back to the shelf as the job leaves, then shows none, rather than the half it made. A folder that holds a video, a sound or a folder is not Kando's alone, and stays; so does that of a done film. The video added without a job is given its folder again when it is next written to, its files beside its video copied in again (libraryAdoptAdded).
func (a *app) libraryHomeUnmade(job core.Job) {
	if job.Home == "" || !job.CompletedAt.IsZero() || job.Redo != "" || !films.InDir(job.LibraryDir, job.Home) || a.lockFilesAway(job.LibraryDir) {
		return
	}
	if info, err := os.Stat(job.Home); err != nil || !info.IsDir() || media.HoldsMedia(job.Home) {
		return
	}
	for video, added := range a.library.AddedIn(job.LibraryDir) {
		if filepath.Clean(added.Home) == filepath.Clean(job.Home) {
			a.library.SetAddedHome(job.LibraryDir, video, "")
		}
	}
	if a.cx == nil {
		return
	}
	trashItem(a.cx, job.Home, func(error) {})
}
