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

// libraryHomeable reports whether a film of the library at root whose video is at video is one whose files Kando keeps in a folder of root (core.Job.Home): root is a library bundle of Kando's own and the video is outside it. A folder of the user's opened as a library keeps the films' files beside their videos, as do the films a bundle holds. Whether the library's folder is there and not locked is libraryHomeAt's to say: a film of one that is not, as one on a disk taken out, is not one whose files go beside its video then, but one none of whose files are written until it is back (core.ErrLibraryGone).
func (a *app) libraryHomeable(root, video string) bool {
	return root != "" && video != "" && shelfBundle(root) && !films.InDir(root, video)
}

// libraryHomeAt is the folder home given a film's files in its library's folder root (core.Job.Home, shelf.Added.Home) as it is now: home when it is there; the folder renamed from it in Finder, found by the [id] in its name (media.RenamedFolder), when the library was not read again since (retargetHomes), so the folder is never made again by its old name beside it; else home made again, as after a job that never completed took it to the Trash, or the user removed it in Finder. root must be there and not locked: the library is never made again here (core.ErrLibraryGone).
func (a *app) libraryHomeAt(root, home string) (string, error) {
	if _, err := os.Stat(home); err == nil {
		return home, nil
	}
	if root == "" || a.lockFilesAway(root) {
		return "", core.ErrLibraryGone
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return "", core.ErrLibraryGone
	}
	if renamed, ok := media.RenamedFolder(root, home); ok {
		return renamed, nil
	}
	if err := os.Mkdir(home, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", err
	}
	return home, nil
}

// libraryAdoptReason is why a film could not be given its folder in its library (libraryAdopt), in the user's words when Kando knows them.
func (a *app) libraryAdoptReason(err error) string {
	if errors.Is(err, core.ErrLibraryGone) {
		return a.messages().LibraryFolderGone()
	}
	return err.Error()
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

// libraryAdopt gives the film of job its folder in its library (core.Job.Home) before anything of it is written, when it is a video kept outside a library bundle that has none yet, and saves the shelf with it: the files a Kando before this one wrote beside the video are copied in first (media.AdoptFiles), its subtitles named there from then on (SubtitlePath), and nothing beside the video changes. A film that has its folder follows it renamed in Finder, or has it made again when it is gone (libraryHomeAt). job is changed only once the shelf is saved: an error leaves it as it was, and the caller does not go on to write, as its files would then be beside the video. A film that is not such a video is left as it is, its files beside its video as always.
func (a *app) libraryAdopt(job *core.Job) error {
	root := job.LibraryDir
	if job.Home != "" {
		home, err := a.libraryHomeAt(root, job.Home)
		if err != nil || home == job.Home {
			return err
		}
		a.libraryHomeRenamed(root, job.Home, home)
		*job = libraryRehomed(*job, job.Home, home)
		return nil
	}
	video := libraryJobVideo(*job)
	if !a.libraryHomeable(root, video) {
		return nil
	}
	next := *job
	next.VideoPath = video
	var placed map[string]string
	undo := func() {}
	// A Kando that knew no Home and ran in between, after a newer one gave the film its folder, kept the subtitles there and dropped the folder from library.json: the folder is the film's again, its files as they are.
	if dir := filepath.Dir(job.SubtitlePath); job.SubtitlePath != "" && filepath.Clean(filepath.Dir(dir)) == filepath.Clean(root) {
		home, err := a.libraryHomeAt(root, dir)
		if err != nil {
			return err
		}
		next = libraryRehomed(next, dir, home)
		next.Home = home
	} else {
		var err error
		if next.Home, placed, undo, err = a.libraryAdoptFiles(root, video, shelf.HomeFor(root, video, job.ID)); err != nil {
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
		// The film goes on reading and writing its files beside its video: copies left in its folder would hide what is written there until it is given the folder.
		undo()
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
		home, err := a.libraryHomeAt(root, added.Home)
		if err != nil {
			return "", err
		}
		if home != added.Home {
			a.libraryHomeRenamed(root, added.Home, home)
		}
		return home, nil
	}
	home, placed, undo, err := a.libraryAdoptFiles(root, video, shelf.HomeFor(root, video, shelf.AddedKey(video)))
	if err != nil {
		return "", err
	}
	a.library.SetAddedHome(root, video, home)
	if err := a.library.Save(); err != nil {
		a.library.SetAddedHome(root, video, "")
		undo()
		return "", err
	}
	a.libraryAddedTakesHome(root, video, home)
	if item, ok := films.Find(a.films(), video); ok {
		a.libraryAdoptedTell(placed, films.Name(item))
	}
	return home, nil
}

// libraryAdoptFiles makes home, the folder of the film's files of the video at video in the library at root, or finds it renamed (libraryHomeAt), and copies the files Kando wrote beside the video into it (media.AdoptFiles): it returns the folder, where each file went, and undo, which takes the copies back.
func (a *app) libraryAdoptFiles(root, video, home string) (string, map[string]string, func(), error) {
	home, err := a.libraryHomeAt(root, home)
	if err != nil {
		return "", nil, nil, err
	}
	placed, undo, err := media.AdoptFiles(video, core.FileBase(video, home), a.savedDevice())
	if err != nil {
		return "", nil, nil, err
	}
	return home, placed, undo, nil
}

// libraryRehomed is job with its folder from renamed to to (core.Job.Home), its subtitles there going with it.
func libraryRehomed(job core.Job, from, to string) core.Job {
	if sub, ok := replacePathPrefix(from, to, job.SubtitlePath); ok {
		job.SubtitlePath = sub
	}
	if job.Home != "" {
		job.Home = to
	}
	return job
}

// libraryRehome names the films of the library at root whose files are in the folder from as having them in to, the folder renamed from it (core.Job.Home, shelf.Added.Home), with their subtitles there: a job and a video added without a job may share one (queueFolderJob). It reports whether any did.
func (a *app) libraryRehome(root, from, to string) bool {
	changed := false
	for i, job := range a.library.Jobs {
		if job.Home != "" && filepath.Clean(job.Home) == filepath.Clean(from) && films.InLibrary(root, job) {
			a.library.Jobs[i] = libraryRehomed(job, from, to)
			changed = true
		}
	}
	for video, added := range a.library.AddedIn(root) {
		if added.Home != "" && filepath.Clean(added.Home) == filepath.Clean(from) {
			a.library.SetAddedHome(root, video, to)
			changed = true
		}
	}
	return changed
}

// libraryHomeRenamed follows the folder of films' files from in the library at root, renamed in Finder to to, as a collection point finds it before the library is read again (libraryHomeAt): library.json and the open library's list name the films' files there at once, so the film being written to and the player follow it.
func (a *app) libraryHomeRenamed(root, from, to string) {
	if !a.libraryRehome(root, from, to) {
		return
	}
	if err := a.library.Save(); err != nil {
		a.saveErr = a.shelfSaveErr(err, a.messages().LibraryShelfSaveFailed)
	}
	a.edit(func(s *session) {
		if filepath.Clean(s.LibraryDir) != filepath.Clean(root) {
			return
		}
		for i := range s.Videos {
			v := &s.Videos[i]
			if v.Home == "" || filepath.Clean(v.Home) != filepath.Clean(from) {
				continue
			}
			if sub, ok := replacePathPrefix(from, to, v.Subtitles); ok {
				v.Subtitles = sub
			}
			v.Home = to
		}
	})
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
			a.warn(a.messages().LibraryAdoptFailed(films.Name(item), a.libraryAdoptReason(err)), "")
			return item
		}
		item.Home, item.SubtitlePath = job.Home, job.SubtitlePath
		if item.VideoPath == "" {
			item.VideoPath = job.VideoPath
		}
	case !item.Added.IsZero():
		home, err := a.libraryAdoptAdded(root, item.VideoPath)
		if err != nil {
			a.warn(a.messages().LibraryAdoptFailed(films.Name(item), a.libraryAdoptReason(err)), "")
			return item
		}
		if home != "" && home != item.Home {
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
	for _, job := range a.library.Jobs {
		if job.Home == "" || !films.InDir(root, job.Home) {
			continue
		}
		if next, ok := media.RenamedFolder(root, job.Home); ok {
			changed = a.libraryRehome(root, job.Home, next) || changed
		}
	}
	for _, added := range a.library.AddedIn(root) {
		if added.Home == "" {
			continue
		}
		if next, ok := media.RenamedFolder(root, added.Home); ok {
			changed = a.libraryRehome(root, added.Home, next) || changed
		}
	}
	return changed
}

// libraryHomeUnmade moves the folder of the files of job's film in its library (core.Job.Home) to the Trash as the job leaves the shelf, when the job never completed (core.Job.CompletedAt) and so made none of the film's subtitles but those it left half made, as one cancelled or failed the first time it made them, and the film leaves the shelf with it. A video added without a job that the job was made of (queueFolderJob), which comes back to the shelf as the job leaves, keeps the folder instead, as its own: what was written into it as the job ran, terms and corrections among them, is the film's, and the half the job made stays as a first making stopped beside the video did; a film open on the job goes on writing there (queueFolderEnded). A folder that holds a video, a sound or a folder is not Kando's alone, and stays; so does that of a done film.
func (a *app) libraryHomeUnmade(job core.Job) {
	if job.Home == "" || !job.CompletedAt.IsZero() || job.Redo != "" || !films.InDir(job.LibraryDir, job.Home) || a.lockFilesAway(job.LibraryDir) {
		return
	}
	if video := libraryJobVideo(job); video != "" {
		if added, ok := a.library.AddedIn(job.LibraryDir)[video]; ok {
			if added.Home == "" {
				a.libraryAddedTakesHome(job.LibraryDir, video, job.Home)
			}
			return
		}
	}
	if info, err := os.Stat(job.Home); err != nil || !info.IsDir() || media.HoldsMedia(job.Home) {
		return
	}
	if a.cx == nil {
		return
	}
	trashItem(a.cx, job.Home, func(error) {})
}

// libraryAddedTakesHome gives the video at video, added to the library at root without a job, home as the folder of its files (shelf.Added.Home), the one its job had, and the open library's list shows it at once, with the subtitles there: the caller saves the shelf.
func (a *app) libraryAddedTakesHome(root, video, home string) {
	a.library.SetAddedHome(root, video, home)
	subtitles := ""
	if base := core.FileBase(video, home); fileExists(media.BilingualPath(base)) {
		subtitles = media.BilingualPath(base)
	}
	a.edit(func(s *session) {
		if filepath.Clean(s.LibraryDir) != filepath.Clean(root) {
			return
		}
		for i := range s.Videos {
			if s.Videos[i].Path == video {
				s.Videos[i].Home, s.Videos[i].Subtitles = home, subtitles
			}
		}
	})
}

// playerHomeFollow reads the open film's lines and info again from its folder in the library once it is given one while it is open, as a paired phone gives one that opened without it (phoneAdopt), or once the folder it has is found renamed (libraryHomeAt): the player names the film's files after its folder (playerUI.home), and would write them where it no longer has them. A film whose folder goes keeps writing where it was, never beside its video. It reports whether it read them again.
func (a *app) playerHomeFollow() bool {
	item, ok := a.openFilm()
	if !ok || item.Home == "" || item.Home == a.player.home {
		return false
	}
	a.player.home = item.Home
	a.player.subs.key = ""
	a.loadSubtitles(item.Key, core.FileBase(films.PlayPath(item), item.Home), item.SubtitlePath)
	if a.player.film.infoFor != "" {
		a.readFilmInfo(a.player.film.infoFor)
	}
	return true
}
