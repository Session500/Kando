package main

import "kando/apps/kando/films"

// Marking films watched or unwatched from their menu (films.App): both act on what the player and the palette keep of how far each video was played, by its path, and save it with the library as playing a film does.

// MarkWatched marks each film of items watched to its end, as playing it to its end would: how far it played becomes its length, which the check over its poster and the palette's search go by, and where it stopped is forgotten, so it opens at its start and the hero no longer offers to continue it. When it last played stays as it was, as marking a film is not playing it. A film whose length is not known yet, whose menu dims the item, or with no video, is left as it is.
func (a *app) MarkWatched(items ...films.Film) {
	marked := false
	for _, item := range items {
		path := item.VideoPath
		length := a.Duration(path)
		if path == "" || length <= 0 {
			continue
		}
		a.playerForget(path, false)
		a.paletteSetFurthest(path, max(length, a.Furthest(path)))
		marked = true
	}
	if marked {
		a.playerPositionsChanged()
	}
}

// MarkUnwatched forgets that each film of items was ever played: where it stopped, how far it played and when, so it shows as a film never played, and the palette finds none of its lines again until it plays. Its subtitle delay and place stay, as they are about its subtitles rather than its watching.
func (a *app) MarkUnwatched(items ...films.Film) {
	marked := false
	for _, item := range items {
		path := item.VideoPath
		if path == "" {
			continue
		}
		a.playerForget(path, true)
		a.paletteSetFurthest(path, 0)
		marked = true
	}
	if marked {
		a.playerPositionsChanged()
	}
}
