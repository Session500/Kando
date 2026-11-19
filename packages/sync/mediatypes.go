package sync

import (
	"mime"
	"path/filepath"
	"strings"
)

// mediaTypes are the media types of the files a desktop serves a phone and puts in the user's storage, by extension: videos, sounds, posters and subtitles. A phone's player tells a file's container by its extension, so the phone names a file it downloads by its type (Extension), and the desktop serves and uploads its files with the type named here (MediaType), as the systems' own tables lack some of them, as m4b, opus and flac on some. An extension that shares its type with one before it, as m4b with m4a, is named by that one on the phone, which plays it the same.
var mediaTypes = []struct{ ext, kind string }{
	{".mp4", "video/mp4"},
	{".m4v", "video/x-m4v"},
	{".mov", "video/quicktime"},
	{".webm", "video/webm"},
	{".mkv", "video/x-matroska"},
	{".avi", "video/x-msvideo"},
	{".flv", "video/x-flv"},
	{".m4a", "audio/mp4"},
	{".m4b", "audio/mp4"},
	{".mp3", "audio/mpeg"},
	{".aac", "audio/aac"},
	{".flac", "audio/flac"},
	{".wav", "audio/wav"},
	{".ogg", "audio/ogg"},
	{".opus", "audio/ogg"},
	{".wma", "audio/x-ms-wma"},
	{".jpg", "image/jpeg"},
	{".png", "image/png"},
	{".webp", "image/webp"},
	{".srt", "application/x-subrip"},
}

// MediaType is the media type of the file at path by its extension: the one mediaTypes names, else the system's, else application/octet-stream.
func MediaType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	for _, t := range mediaTypes {
		if t.ext == ext {
			return t.kind
		}
	}
	if ext != "" {
		if kind := mime.TypeByExtension(ext); kind != "" {
			return kind
		}
	}
	return "application/octet-stream"
}

// Extension is the file extension of a file served as content type kind, a Content-Type header's value, else fallback.
func Extension(kind, fallback string) string {
	t, _, err := mime.ParseMediaType(kind)
	if err != nil {
		return fallback
	}
	t = strings.ToLower(t)
	for _, m := range mediaTypes {
		if m.kind == t {
			return m.ext
		}
	}
	return fallback
}
