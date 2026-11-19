package media

import (
	"os"
	"strings"

	"kando/packages/core"
)

// exportSubtitles writes a film's subtitles as Kando wrote them before its document, a film made whole: original to its transcript (TranscriptOf) and "<video>.bilingual.srt" with each translation above its original line, or the original alone for a line left untranslated, returning the bilingual path. Tests make the films of earlier versions with it.
func exportSubtitles(videoPath string, original, translated []core.Segment) (string, error) {
	if err := saveTranscript(TranscriptOf(videoPath), original); err != nil {
		return "", err
	}
	both := make([]core.Segment, len(original))
	for i, s := range original {
		both[i] = s
		if i < len(translated) {
			if text := strings.TrimSpace(translated[i].Text); text != "" {
				both[i].Text = text + "\n" + s.Text
			}
		}
	}
	path := strings.TrimSuffix(videoPath, ".mp4")
	if i := strings.LastIndex(videoPath, "."); i > strings.LastIndex(videoPath, "/") {
		path = videoPath[:i]
	}
	path += ".bilingual.srt"
	return path, os.WriteFile(path, []byte(formatSRT(both)), 0o644)
}

// loadTranscript reads back the original speech saveTranscript wrote to path.
func loadTranscript(path string) ([]core.Segment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseSRT(data)
}
