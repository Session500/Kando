package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"kando/apps/kando/films"
	"kando/apps/kando/shelf"
	"kando/packages/core"
	"kando/packages/media"
)

// A video kept outside a library bundle, where the user picked it, has a folder of its own in the bundle that Kando writes the film's files in (core.Job.Home, shelf.Added.Home), so nothing is ever written into the user's folder. A film is given its folder, and the files a Kando before this one wrote beside its video are copied in (libraryAdopt), at the only places that come before anything of it is written: as the player opens it (Open), as the queue starts its job (queueRun) and as a paired phone writes to it (phoneAdopt). Until then it reads its files beside its video, as it always did, and nothing writes them.

// libraryHomeFor is the folder the own files of the film of the video at video, of the library at root, are kept in once it is given one (core.Job.Home, shelf.Added.Home), key naming one kept outside the library (shelf.HomeFor): in a library bundle, a folder of the bundle's for a video kept outside it; in a folder of the user's opened as a library, the hidden folder beside a video it holds (media.FolderHome, beside reports it), and for one kept outside it a folder in the hidden folder at its top (media.FilesDir), as a bundle's. "" for a video a bundle holds, whose files are beside it. Whether the library's folder is there and not locked is libraryHomeAt's and libraryAdoptBeside's to say: a film of one that is not, as one on a disk taken out, is not one whose files go beside its video then, but one none of whose files are written until it is back (core.ErrLibraryGone).
func libraryHomeFor(root, video, key string) (home string, beside bool) {
	if root == "" || video == "" {
		return "", false
	}
	if home := media.FolderHomeFor(root, video); home != "" {
		return home, true
	}
	if films.InDir(root, video) {
		return "", false
	}
	if shelfBundle(root) {
		return shelf.HomeFor(root, video, key), false
	}
	return shelf.HomeFor(filepath.Join(root, media.FilesDir), video, key), false
}

// libraryHomeable reports whether a film of the library at root whose video is at video is one whose files Kando keeps in a folder of their own (libraryHomeFor), not beside its video.
func (a *app) libraryHomeable(root, video string) bool {
	home, _ := libraryHomeFor(root, video, "")
	return home != ""
}

// libraryRootReady says why nothing of a film of the library at root can be written now: its folder is not there, or it is locked (core.ErrLibraryGone); nil when it can.
func (a *app) libraryRootReady(root string) error {
	if root == "" || a.lockFilesAway(root) {
		return core.ErrLibraryGone
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return core.ErrLibraryGone
	}
	return nil
}

// libraryAdoptBeside gives the film of the video at video, of the folder of the user's at root opened as a library, its folder beside the video (media.FolderHome, media.AdoptInto), copying in what a Kando before this one wrote beside the video, once: a folder that is there already, given on this computer or another sharing the library, is left as it is. It returns where each file went, and the message bar says so once (libraryAdoptedTell) under name.
func (a *app) libraryAdoptBeside(root, video, home, name string) error {
	if err := a.libraryRootReady(root); err != nil {
		return err
	}
	placed, _, err := media.AdoptInto(video, home, a.savedDevice())
	if err != nil {
		return err
	}
	a.libraryAdoptedTell(placed, name)
	return nil
}

// libraryHomeAt is the folder home given a film's files in its library's folder root (core.Job.Home, shelf.Added.Home) as it is now: home when it is there; the folder renamed from it in Finder, found by the [id] in its name (media.RenamedFolder), when the library was not read again since (retargetHomes), so the folder is never made again by its old name beside it; else home made again, as after a job that never completed took it to the Trash, or the user removed it in Finder. root must be there and not locked: the library is never made again here (core.ErrLibraryGone).
func (a *app) libraryHomeAt(root, home string) (string, error) {
	if _, err := os.Stat(home); err == nil {
		return home, nil
	}
	if err := a.libraryRootReady(root); err != nil {
		return "", err
	}
	// A folder of the user's opened as a library keeps the folders of the videos kept outside it in its hidden folder at its top (media.FilesDir), made as the first is.
	store := filepath.Dir(home)
	if filepath.Base(store) == media.FilesDir && filepath.Clean(filepath.Dir(store)) == filepath.Clean(root) {
		if err := os.Mkdir(store, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
	if renamed, ok := media.RenamedFolder(store, home); ok {
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
	video := libraryJobVideo(*job)
	if job.Home != "" {
		// A folder beside the video that is gone, as the user took it out in Finder, is given again from beside the video, as it first was: one made empty would count as given.
		if filepath.Clean(job.Home) == filepath.Clean(media.FolderHome(video)) {
			return a.libraryAdoptBeside(root, video, job.Home, a.libraryJobName(*job))
		}
		home, err := a.libraryHomeAt(root, job.Home)
		if err != nil || home == job.Home {
			return err
		}
		a.libraryHomeRenamed(root, job.Home, home)
		*job = libraryRehomed(*job, job.Home, home)
		return nil
	}
	home, beside := libraryHomeFor(root, video, job.ID)
	if home == "" {
		return nil
	}
	next := *job
	next.VideoPath = video
	var placed map[string]string
	undo := func() {}
	switch {
	case beside:
		if err := a.libraryAdoptBeside(root, video, home, a.libraryJobName(*job)); err != nil {
			return err
		}
		next.Home = home
		next.SubtitlePath = libraryAdoptedPath(job.SubtitlePath, video, next.FileBase(), nil)
	// A Kando that knew no Home and ran in between, after a newer one gave the film its folder, kept the subtitles there and dropped the folder from library.json: the folder is the film's again, its files as they are.
	case shelfBundle(root) && job.SubtitlePath != "" && filepath.Clean(filepath.Dir(filepath.Dir(job.SubtitlePath))) == filepath.Clean(root):
		dir := filepath.Dir(job.SubtitlePath)
		home, err := a.libraryHomeAt(root, dir)
		if err != nil {
			return err
		}
		next = libraryRehomed(next, dir, home)
		next.Home = home
	default:
		var err error
		if next.Home, placed, undo, err = a.libraryAdoptFiles(root, video, home); err != nil {
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
	home, beside := libraryHomeFor(root, video, shelf.AddedKey(video))
	if !ok || home == "" {
		return "", nil
	}
	// One the folder of the user's opened as a library holds keeps its files in the folder beside it, as a video found there does.
	if beside {
		if err := a.libraryAdoptBeside(root, video, home, a.libraryVideoName(video)); err != nil {
			return "", err
		}
		if added.Home != home {
			a.libraryAddedTakesHome(root, video, home)
			if err := a.library.Save(); err != nil {
				a.saveErr = a.shelfSaveErr(err, a.messages().LibraryShelfSaveFailed)
			}
		}
		return home, nil
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
	home, placed, undo, err := a.libraryAdoptFiles(root, video, home)
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
	a.libraryAdoptedTell(placed, a.libraryVideoName(video))
	return home, nil
}

// libraryAdoptFound gives the video at video, found in the folder of the user's at root opened as a library, that no job holds nor was added without one, its folder beside it (libraryAdoptBeside), and the open library's list shows it at once. It keeps no record of it: the folder there is the record (media.AdoptInto), as the library lists it (media.ListVideos). "" for a video whose files stay beside it, as one a bundle holds.
func (a *app) libraryAdoptFound(root, video string) (string, error) {
	home := media.FolderHomeFor(root, video)
	if home == "" {
		return "", nil
	}
	if err := a.libraryAdoptBeside(root, video, home, a.libraryVideoName(video)); err != nil {
		return "", err
	}
	a.librarySessionHome(root, video, home)
	return home, nil
}

// libraryVideoName is the name the shelf gives the film of the video at video, its file's name when the open library does not list it.
func (a *app) libraryVideoName(video string) string {
	if item, ok := films.Find(a.films(), video); ok {
		return films.Name(item)
	}
	return strings.TrimSuffix(filepath.Base(video), filepath.Ext(video))
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
	default:
		adopt := a.libraryAdoptFound
		if !item.Added.IsZero() {
			adopt = a.libraryAdoptAdded
		}
		home, err := adopt(root, item.VideoPath)
		if err != nil {
			a.warn(a.messages().LibraryAdoptFailed(films.Name(item), a.libraryAdoptReason(err)), "")
			return item
		}
		if home != "" && home != item.Home {
			if refreshed, ok := films.Find(a.films(), item.Key); ok {
				item = refreshed
			} else {
				item.Home = home
			}
		}
	}
	a.libraryHomeVideoChanged(item)
	return item
}

// libraryHomeVideoChanged says once, in the message bar, that the file of the film item opens is not the one its folder beside it was given for (media.FolderVideoChanged), as one the user put in its place under the same name in Finder, another episode or another cut: its subtitles may be the other one's. The file is kept as the folder's from then on, so it is said once.
func (a *app) libraryHomeVideoChanged(item films.Film) {
	if item.Home == "" || item.VideoPath == "" || filepath.Clean(item.Home) != filepath.Clean(media.FolderHome(item.VideoPath)) || !media.FolderVideoChanged(item.Home, item.VideoPath) {
		return
	}
	if err := media.KeepFolderVideo(item.Home, item.VideoPath); err != nil {
		return
	}
	a.warn(a.messages().LibraryHomeOtherVideo(films.Name(item)), "")
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
	adopt := a.libraryAdoptFound
	if _, added := a.library.AddedIn(dir)[video]; added {
		adopt = a.libraryAdoptAdded
	}
	home, err := adopt(dir, video)
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
	// A folder beside a video of a folder of the user's opened as a library is its film's whatever job made of it: the video stays in the library, found there, with what was written into its folder.
	if video := libraryJobVideo(job); video != "" && filepath.Clean(job.Home) == filepath.Clean(media.FolderHome(video)) {
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
	a.librarySessionHome(root, video, home)
}

// librarySessionHome shows the film of the video at video in the open library's list, when root is that library, with its folder home and the subtitles there.
func (a *app) librarySessionHome(root, video, home string) {
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
