package main

import (
	"slices"
	"strings"
	"time"

	"kando/apps/kando/film"
	"kando/apps/kando/films"
	"kando/apps/kando/live"
	"kando/apps/kando/locales"
	"kando/apps/kando/playlist"
	"kando/apps/kando/subscribe"
	kcore "kando/packages/core"

	ns "github.com/nsgo-dev/nsgo/core"
)

// What plays after the open film (films.Next): the list a film was added from, kept on its job (core.Job.List), and the card that plays the next once the film ends.

// nextKeepLists notes, on the jobs of the open library whose links are videos of lists, as the playlist dialog read them, the list each came from and its place in it (core.Job.List): a playlist's own order, which a playlist read again writes anew, as its owner may have moved its videos; a channel's from its oldest video, which a channel lists last, its videos already placed keeping their places and its new ones placed after them. A list that is one work, a playlist, or a channel series says is (as the user said a list they subscribed to is a series), gives its jobs its title too (core.Job.ListTitle), so the library shows them as one card; a channel's videos are a channel's, not one work, and keep none.
func (a *app) nextKeepLists(lists []playlist.List, series bool) {
	root := a.snap().LibraryDir
	changed := false
	for _, list := range lists {
		key := queueSourceKey(list.Source)
		place := map[string]int{}
		if list.Channel {
			last := a.nextLastPlace(root, key)
			for i := len(list.Videos) - 1; i >= 0; i-- {
				video := queueSourceKey(list.Videos[i].URL)
				if a.nextPlaced(root, key, video) {
					continue
				}
				last++
				place[video] = last
			}
		} else {
			for i, v := range list.Videos {
				place[queueSourceKey(v.URL)] = i + 1
			}
		}
		title := ""
		if !list.Channel || series {
			title = strings.TrimSpace(list.Title)
		}
		for _, job := range a.library.Jobs {
			if !films.InLibrary(root, job) {
				continue
			}
			at, ok := place[queueSourceKey(job.Source)]
			switch {
			case ok && (job.List != key || job.ListIndex != at || job.ListTitle != title):
				job.List, job.ListIndex, job.ListTitle = key, at, title
			// A video of a channel placed before keeps its place, and takes the title the list has now.
			case !ok && job.List == key && job.ListTitle != title:
				job.ListTitle = title
			default:
				continue
			}
			a.library.Upsert(job)
			changed = true
		}
	}
	if changed {
		a.nextSave()
	}
}

// nextKeepSubscribed notes, on the jobs of sub's library for videos it has just queued, oldest first, the list they came from, sub's, and their places in it, after those of the list's videos placed already, as a subscription adds what comes out after. Every job of the list takes the subscription's name as its title when the list is one work, a playlist's or a collection's, or a channel the user said is a series (core.Job.ListTitle), and none once the user says it is not, so the library folds its films into one card as long as it is.
func (a *app) nextKeepSubscribed(sub *subscribe.Sub, queued []subscribe.Video) {
	key := queueSourceKey(sub.Source)
	last := a.nextLastPlace(sub.Library, key)
	title := ""
	if !sub.Channel() || sub.Series {
		title = strings.TrimSpace(sub.Name)
	}
	changed := false
	for _, v := range queued {
		video := queueSourceKey(v.URL)
		for _, job := range a.library.Jobs {
			if job.List != "" || !films.InLibrary(sub.Library, job) || queueSourceKey(job.Source) != video {
				continue
			}
			last++
			job.List, job.ListIndex = key, last
			a.library.Upsert(job)
			changed = true
			break
		}
	}
	for _, job := range a.library.Jobs {
		if job.List == key && job.ListTitle != title && films.InLibrary(sub.Library, job) {
			job.ListTitle = title
			a.library.Upsert(job)
			changed = true
		}
	}
	if changed {
		a.nextSave()
	}
}

// nextLastPlace is the last place of list key among the jobs of the library at root, 0 for a list none is placed in.
func (a *app) nextLastPlace(root, key string) int {
	last := 0
	for _, job := range a.library.Jobs {
		if job.List == key && films.InLibrary(root, job) {
			last = max(last, job.ListIndex)
		}
	}
	return last
}

// nextPlaced reports whether the library at root has a job for video, a link's key, placed in list key.
func (a *app) nextPlaced(root, key, video string) bool {
	for _, job := range a.library.Jobs {
		if job.List == key && job.ListIndex > 0 && queueSourceKey(job.Source) == video && films.InLibrary(root, job) {
			return true
		}
	}
	return false
}

// nextSave writes library.json once lists were noted on its jobs.
func (a *app) nextSave() {
	if err := a.library.Save(); err != nil {
		a.saveErr = a.shelfSaveErr(err, a.messages().LibraryShelfSaveFailed)
	}
}

// nextAfter is how long the card counts down before the next film plays, time to read what it is and stay.
const nextAfter = 8 * time.Second

// playerNext is the card that plays the next film once the open one ends (film.App.UpNext): the playback that ended and its seek count, which it shows for once, so a card put away does not come back while the film stays at its end; the next film's key; when it plays, zero while it waits to be made; and the timer that plays it. preferredFor is the film whose next was put first in the queue (playerNextPrefer).
type playerNext struct {
	play         *filmPlay
	seeks        int
	open         bool
	key          string
	due          time.Time
	stop         func() bool
	preferredFor string
}

// playerNextFollow, after each change, shows the card once the open film has played to its end, puts it away once the film plays or seeks again, another opens or the user looks up a word or corrects a line, and starts its countdown once the next film can play. It reports whether that changed what the page shows.
func (a *app) playerNextFollow() bool {
	n := &a.player.next
	changed := a.playerNextPrefer()
	p := a.player.play
	if n.open {
		if p == nil || p != n.play || p.seeks != n.seeks || !p.finished || a.vocab.lookup.open || a.correctSelecting() {
			a.playerNextClose()
			return true
		}
		next, ok := a.playerNextFilm()
		switch {
		case !ok:
			a.playerNextClose()
			return true
		case next.Key != n.key:
			n.key, n.due = next.Key, time.Time{}
			a.playerNextStop()
			changed = true
		}
		if n.due.IsZero() && playerNextPlayable(next) {
			a.playerNextCount()
			changed = true
		}
		return changed
	}
	if p == nil || !p.finished || (p == n.play && p.seeks == n.seeks) || a.player.nextOff || a.LoopLine() {
		return changed
	}
	n.play, n.seeks = p, p.seeks
	next, ok := a.playerNextFilm()
	if !ok {
		return changed
	}
	n.open, n.key, n.due = true, next.Key, time.Time{}
	if playerNextPlayable(next) {
		a.playerNextCount()
	}
	return true
}

// playerNextPlayable reports whether the next film can play now: done, its video kept or streamed, or being made with its video already there (films.Watchable). One still waiting or downloading is waited for.
func playerNextPlayable(item films.Film) bool {
	return item.Stage == kcore.StageComplete || films.Watchable(item)
}

// playerNextFilm is the film that plays after the open one (films.Next), among the open library's films.
func (a *app) playerNextFilm() (films.Film, bool) {
	item, ok := a.openFilm()
	if !ok {
		return films.Film{}, false
	}
	return films.Next(a.films(), item, a.prefs.interfaceLanguage)
}

// playerNextCount starts the countdown to the next film.
func (a *app) playerNextCount() {
	n := &a.player.next
	a.playerNextStop()
	n.due = a.playerNow().Add(nextAfter)
	if a.cx != nil {
		n.stop = a.cx.AfterFunc(nextAfter, live.Do(a, a.playerNextDue))
	}
}

// playerNextDue is the countdown ending: the next film plays, if the card still shows for it and it is still the next.
func (a *app) playerNextDue() {
	n := &a.player.next
	n.stop = nil
	if !n.open || n.due.IsZero() {
		return
	}
	next, ok := a.playerNextFilm()
	if !ok || next.Key != n.key || !playerNextPlayable(next) {
		a.playerNextClose()
		return
	}
	a.playerNextOpen(next)
}

// playerNextOpen plays item, from where it stopped last time (films.ResumeFrom), as the hero's 继续 does.
func (a *app) playerNextOpen(item films.Film) {
	a.playerNextClose()
	at, _ := films.ResumeFrom(a, item)
	a.OpenAt(item, at)
}

// playerNextStop stops the countdown's timer.
func (a *app) playerNextStop() {
	if n := &a.player.next; n.stop != nil {
		n.stop()
		n.stop = nil
	}
}

// playerNextClose puts the card away, for the playback that ended until it plays or seeks again.
func (a *app) playerNextClose() {
	a.playerNextStop()
	n := &a.player.next
	n.open, n.key, n.due = false, "", time.Time{}
}

// playerNextPrefer puts the next film of the film opened right after it in the queue, once for each film opened, when it waits to be made, so it is made by the time the opened one ends; the job running is not stopped for it. It reports whether that changed the queue.
func (a *app) playerNextPrefer() bool {
	n := &a.player.next
	if a.openKey == "" || a.openKey == n.preferredFor {
		return false
	}
	n.preferredFor = a.openKey
	next, ok := a.playerNextFilm()
	if !ok || next.JobID == "" || next.Stage != kcore.StageQueued || next.JobID == a.queue.active {
		return false
	}
	at := slices.Index(a.queue.order, next.JobID)
	if at < 0 {
		return false
	}
	order := slices.Delete(slices.Clone(a.queue.order), at, at+1)
	a.queue.order = slices.Insert(order, min(a.queueAfterPreferred(next.JobID), len(order)), next.JobID)
	a.pump()
	return true
}

// UpNext is the card as the page shows it (film.Nexter).
func (a *app) UpNext() film.UpNext {
	n := a.player.next
	if !n.open {
		return film.UpNext{}
	}
	item, ok := films.Find(a.films(), n.key)
	if !ok {
		return film.UpNext{}
	}
	up := film.UpNext{Open: true, Title: a.nextTitle(item), Due: n.due, Countdown: nextAfter}
	up.Poster = a.posterOf(item.VideoPath)
	if item.VideoPath == "" {
		up.Poster = a.thumbnailOf(item)
	}
	if n.due.IsZero() {
		up.Waiting = func(cx *ns.Scope) string { return films.QueueMeta(locales.For(cx), item) }
	}
	return up
}

// PlayNext plays the next film now, as the card's 立即播放, Return on it and 下一部 do; without a card it plays the open film's next, from the menu or ⇧N.
func (a *app) PlayNext() {
	next, ok := a.playerNextFilm()
	if !ok {
		return
	}
	a.playerNextOpen(next)
}

// CancelNext puts the card away, as its 取消 and Escape do: the film stays at its end.
func (a *app) CancelNext() { a.playerNextClose() }

// PlayPrevious plays the film before the open one (films.Previous), from where it stopped, as ⇧P does; past the first seconds of the open film, or with none before it, it plays the open film from its start, as a music player's previous does.
func (a *app) PlayPrevious() {
	item, ok := a.openFilm()
	if !ok {
		return
	}
	prev, ok := films.Previous(a.films(), item, a.prefs.interfaceLanguage)
	if !ok || a.PlayHead() > playerPreviousRestart {
		if a.playerLoaded() {
			a.seekPlayback(0)
		}
		return
	}
	a.playerNextOpen(prev)
}

// playerPreviousRestart is how far into the open film ⇧P plays it from its start rather than the film before it.
const playerPreviousRestart = 3 * time.Second

// HasNext reports whether the open film has a film after it, which 下一部 plays.
func (a *app) HasNext() bool {
	_, ok := a.playerNextFilm()
	return ok
}

// AutoNext reports whether the next film plays by itself once the open one ends (自动播放下一部), and ToggleAutoNext turns that on or off and saves it.
func (a *app) AutoNext() bool { return !a.player.nextOff }

func (a *app) ToggleAutoNext() {
	a.player.nextOff = !a.player.nextOff
	if a.player.nextOff {
		a.playerNextClose()
	}
	a.playerSaveSettings()
}

// nextTitle is what the Up Next card calls item: its name, after its work's when play goes on into another work than the open film's, as from a course's last lesson into the folder beside it, so the card says it is leaving the work.
func (a *app) nextTitle(item films.Film) string {
	works := a.libraryWorks()
	next, ok := works.Of(item.Key)
	if !ok {
		return films.Name(item)
	}
	if open, ok := works.Of(a.openKey); ok && open == next {
		return films.Name(item)
	}
	return a.messages().FilmNextInWork(next.Name, films.Name(item))
}
