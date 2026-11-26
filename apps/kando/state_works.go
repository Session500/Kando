package main

import (
	"kando/apps/kando/films"
	"kando/packages/study"
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

// worksApartKey is what the open library's categories keep a work the user split apart by (category.Book.Apart): its list's, or its folder within the library, as its films are kept there (study.KeyIn), with its show's, so it stays split as the library moves.
func (a *app) worksApartKey(w *films.Work) string {
	if w.List != "" {
		return "list\t" + w.List
	}
	return "show\t" + study.KeyIn(a.snap().LibraryDir, w.Folder) + "\t" + w.Show
}

func (a *app) WorkApart(w *films.Work) bool {
	return w != nil && a.cats.book.Apart(a.worksApartKey(w))
}

// SetWorkApart shows w's films a card each, or one card again, and keeps that with the library's categories. A library whose categories cannot be changed, as a locked one, leaves it as it was.
func (a *app) SetWorkApart(w *films.Work, apart bool) {
	book, ok := a.categoryBook()
	if !ok || w == nil || !book.SetApart(a.worksApartKey(w), apart) {
		return
	}
	a.categoryWrite()
}
