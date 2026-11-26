package media

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"kando/packages/core"
)

// chapterLine is a chapter as ffmpeg -i lists it, "Chapter #0:1: start 4.000000, end 8.000000", and chapterTitle the title in the metadata under it, "title           : Two".
var (
	chapterLine  = regexp.MustCompile(`^\s*Chapter #\d+:\d+: start (-?\d+(?:\.\d+)?), end`)
	chapterTitle = regexp.MustCompile(`^\s*title\s*: ?(.*)$`)
)

// EmbeddedChapters are the chapters a media file marks in itself, as an audiobook's m4b or a film's mkv does, as ffmpeg reads them from its header: none for a file that marks fewer than two (core.CleanChapters), or that ffmpeg cannot read.
func EmbeddedChapters(ffmpeg, path string) []core.Chapter {
	if ffmpeg == "" || path == "" {
		return nil
	}
	return parseChapters(ffmpegStderr(context.Background(), ffmpeg, "-hide_banner", "-nostdin", "-i", path))
}

// EmbeddedChapters are the chapters the media file at path marks in itself (media.EmbeddedChapters), which Make keeps for a local film, whose site marks none.
func (t Tools) EmbeddedChapters(_ context.Context, path string) []core.Chapter {
	return EmbeddedChapters(t.FFmpeg, path)
}

// EmbeddedChapterer is a pipeline that reads the chapters a media file marks in itself, as Tools does. Make keeps them for a local film.
type EmbeddedChapterer interface {
	EmbeddedChapters(ctx context.Context, path string) []core.Chapter
}

// KeepEmbeddedChapters is the chapters of the film whose file is at path and whose files are named after base (core.FileBase), those kept with them, else the ones its file marks in itself, read with ffmpeg and kept with its files from then on (keepChapters), as for a film made before Kando read them, whose cut and titles then work as a site's chapters do. ok false for a film with none.
func KeepEmbeddedChapters(ffmpeg, path, base string) (FilmChapters, bool) {
	if kept, ok := ReadChapters(base); ok {
		return kept, true
	}
	found := core.CleanChapters(EmbeddedChapters(ffmpeg, path))
	if len(found) == 0 {
		return FilmChapters{}, false
	}
	keepChapters(base, found, "")
	if kept, ok := ReadChapters(base); ok {
		return kept, true
	}
	// A folder that cannot be written to keeps nothing, and the film shows the chapters it marks all the same.
	return FilmChapters{Chapters: found}, true
}

// parseChapters reads the chapters out of what ffmpeg -i writes of a file, each named by its title, or by its number without one.
func parseChapters(text string) []core.Chapter {
	var out []core.Chapter
	inChapter := false
	for _, line := range strings.Split(text, "\n") {
		if m := chapterLine.FindStringSubmatch(line); m != nil {
			start, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				inChapter = false
				continue
			}
			out = append(out, core.Chapter{Start: seconds(max(start, 0))})
			inChapter = true
			continue
		}
		if strings.Contains(line, "Stream #") {
			inChapter = false
		}
		if m := chapterTitle.FindStringSubmatch(line); inChapter && m != nil && out[len(out)-1].Title == "" {
			out[len(out)-1].Title = strings.TrimSpace(m[1])
		}
	}
	// A chapter without a title, as many a file marks, is named by its number, so it is not dropped and its time taken by the one before.
	for i := range out {
		if strings.TrimSpace(out[i].Title) == "" {
			out[i].Title = strconv.Itoa(i + 1)
		}
	}
	return core.CleanChapters(out)
}
