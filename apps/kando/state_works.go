package main

import (
	"slices"

	"kando/apps/kando/category"
	"kando/apps/kando/films"
	kcore "kando/packages/core"
)

// worksUI is the open library's works (films.Works), whose films the library page folds into one card each, and the work the page shows the films of.
type worksUI struct {
	// cache keeps the works, worked out again only once the films they are made of change (films.WorksCache).
	cache films.WorksCache
	// open is the key of the work the library page shows the films of (films.App.OpenWork), "" while it shows the library's. Like the sidebar's filters it is not saved, and another library starts without it.
	open string
}

// worksHooks: another library opens on its films, not on a work of the last one.
func (a *app) worksHooks() areaHooks {
	return areaHooks{libraryChanged: func() { a.works.open = "" }}
}

// libraryWorks is the open library's works, as the interface's language orders their names. UI thread only.
func (a *app) libraryWorks() films.WorkIndex {
	return a.works.cache.Get(a.films(), a.snap().LibraryDir, a.prefs.interfaceLanguage)
}

func (a *app) Works() films.WorkIndex { return a.libraryWorks() }

func (a *app) OpenWork() string { return a.works.open }

func (a *app) ShowWork(key string) { a.works.open = key }

func (a *app) WorkApart(w *films.Work) bool {
	return w != nil && a.cats.book.Apart(w.ApartKey(a.snap().LibraryDir))
}

// SetWorkApart shows w's films a card each, or one card again, and keeps that with the library's categories. A library whose categories cannot be changed, as a locked one, leaves it as it was.
func (a *app) SetWorkApart(w *films.Work, apart bool) {
	book, ok := a.categoryBook()
	root := a.snap().LibraryDir
	if !ok || w == nil || !book.SetApart(w.ApartKey(root), apart) {
		return
	}
	a.categoryWrite()
	// The phones show the work as the library does (phoneSource.Films), and list its films again now rather than at their next read.
	if srv := a.phone.server; srv != nil {
		srv.PublishFilms(phoneLibraryID(root))
	}
}

// worksInherit puts job's film, the new episode of a work its job has just made, into the categories every episode of its work before it that is done is in, as a show's new episode joins 动漫 with the rest: only the categories they all share, so one lesson put in 难点 alone does not take the ones after it there. A film of no work, the first of its work, or of a library other than the one open, whose films are not at hand, goes into none; nor does one of a library whose categories cannot be changed now, as a locked one. It is done once, as the film is made, so a film the user takes out of a category stays out.
func (a *app) worksInherit(job kcore.Job) {
	root := a.snap().LibraryDir
	if root == "" || !films.InLibrary(root, job) || a.lockFilesAway(root) {
		return
	}
	book, ok := a.categoryBook()
	if !ok {
		return
	}
	all := a.films()
	w, ok := a.libraryWorks().Of(job.ID)
	if !ok {
		return
	}
	var shared []string
	first := true
	for _, key := range w.Films[:slices.Index(w.Films, job.ID)] {
		f, ok := films.Find(all, key)
		if !ok || f.Stage != kcore.StageComplete || f.VideoPath == "" {
			continue
		}
		in := book.Of(f.VideoPath)
		if first {
			shared, first = slices.Clone(in), false
			continue
		}
		shared = slices.DeleteFunc(shared, func(id string) bool { return !slices.Contains(in, id) })
	}
	changed := false
	for _, id := range shared {
		changed = book.Put(id, true, job.VideoPath) || changed
	}
	if changed {
		if err := book.Save(); err != nil && err != category.ErrUnread {
			a.saveErr = a.messages().CategorySaveFailed(err.Error())
		}
	}
}
