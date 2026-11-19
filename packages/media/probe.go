package media

import (
	"regexp"
	"strconv"
	"time"
)

// durationPattern is how long ffmpeg -i says a file plays, as "Duration: 00:01:05.00".
var durationPattern = regexp.MustCompile(`Duration: (\d+):(\d+):(\d+(?:\.\d+)?)`)

// Duration is how long the media file at path plays, as its header says when ffmpeg reads it, with or without a picture; 0 when ffmpeg cannot tell. Tools.Length decodes the file when the header does not say.
func Duration(ffmpeg, path string) time.Duration {
	if ffmpeg == "" || path == "" {
		return 0
	}
	return ReadHeader(ffmpeg, path).Length
}

func parseDuration(text string) time.Duration {
	match := durationPattern.FindStringSubmatch(text)
	if match == nil {
		return 0
	}
	hours, _ := strconv.Atoi(match[1])
	minutes, _ := strconv.Atoi(match[2])
	seconds, _ := strconv.ParseFloat(match[3], 64)
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds*float64(time.Second))
}
