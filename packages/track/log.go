package track

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// ErrDevice is the error of writing with a device id that cannot name a file: empty, or holding anything but letters, digits, '-' and '_'.
var ErrDevice = errors.New("track: device id cannot name a log")

// ErrNewer is the error of writing to a film whose logs a newer Kando wrote (Doc.Newer), which this one would not keep the records of.
var ErrNewer = errors.New("track: the film's subtitles were written by a newer Kando")

const logSuffix = ".jsonl"

// logPrefix is how the name of every log of the video at videoPath begins: a hidden file beside it, as ".video.track.".
func logPrefix(videoPath string) string {
	name := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	return "." + name + ".track."
}

// LogPath is the log device keeps of the video at videoPath: ".video.track.<device>.jsonl" beside it.
func LogPath(videoPath, device string) string {
	return filepath.Join(filepath.Dir(videoPath), logPrefix(videoPath)+device+logSuffix)
}

// Logs are the logs every device keeps of the video at videoPath, as LogPath names them, sorted by name; none when it has none or its folder cannot be read.
func Logs(videoPath string) []string {
	entries, err := os.ReadDir(filepath.Dir(videoPath))
	if err != nil {
		return nil
	}
	prefix := logPrefix(videoPath)
	var out []string
	for _, e := range entries {
		name := e.Name()
		device, ok := strings.CutPrefix(name, prefix)
		if !ok || !e.Type().IsRegular() {
			continue
		}
		if device, ok = strings.CutSuffix(device, logSuffix); ok && validDevice(device) {
			out = append(out, filepath.Join(filepath.Dir(videoPath), name))
		}
	}
	slices.Sort(out)
	return out
}

// deviceOf is the device whose log is at path, as LogPath names it.
func deviceOf(videoPath, path string) string {
	device := strings.TrimPrefix(filepath.Base(path), logPrefix(videoPath))
	return strings.TrimSuffix(device, logSuffix)
}

func validDevice(device string) bool {
	if device == "" {
		return false
	}
	for _, r := range device {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// tail is what Append knows of the end of a log it wrote: its last record's Seq and At, and the size and time of the file once written, which say whether another program changed it since.
type tail struct {
	seq, at int64
	size    int64
	mod     time.Time
}

var (
	logMu sync.Mutex
	tails = map[string]tail{}
)

// Now is the clock Append stamps records with; tests set it.
var Now = time.Now

// Append adds events, in order, to the log device keeps of the video at videoPath (LogPath), numbering and stamping each (Event.Seq, Event.At), with a head first when the log is new, and returns the records it wrote, as Read would read them back, so a writer can keep the document in memory without reading it again. A record cut off at the end of the log, as by a crash while it was written, is dropped first, so the records written now follow whole ones. Records are written together, so a crash leaves a log of whole records and at most one cut off at its end, which Read skips.
func Append(videoPath, device string, events ...Event) ([]Event, error) {
	if !validDevice(device) {
		return nil, ErrDevice
	}
	if len(events) == 0 {
		return nil, nil
	}
	path := LogPath(videoPath, device)
	logMu.Lock()
	defer logMu.Unlock()
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	t, err := tailOf(path, f)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	written := make([]Event, 0, len(events)+1)
	if t.seq == 0 {
		events = append([]Event{{Kind: KindHead, Version: Version}}, events...)
	}
	for _, e := range events {
		t.seq++
		t.at = max(Now().UnixMilli(), t.at+1)
		e.Seq, e.At, e.Device = t.seq, t.at, ""
		data, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		buf.Write(data)
		buf.WriteByte('\n')
		e.Device = device
		written = append(written, e)
	}
	if _, err := f.Seek(t.size, io.SeekStart); err != nil {
		return nil, err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		delete(tails, path)
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		delete(tails, path)
		return written, nil
	}
	t.size, t.mod = info.Size(), info.ModTime()
	tails[path] = t
	return written, nil
}

// tailOf is the end of the log at path, open as f: what Append kept of it when the file is as Append left it, or else read from the file, which is cut back to its last whole record. A log that is not this format's, or that a newer Kando wrote, is ErrNewer.
func tailOf(path string, f *os.File) (tail, error) {
	info, err := f.Stat()
	if err != nil {
		return tail{}, err
	}
	if t, ok := tails[path]; ok && t.size == info.Size() && t.mod.Equal(info.ModTime()) {
		return t, nil
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return tail{}, err
	}
	whole := bytes.LastIndexByte(data, '\n') + 1
	if whole < len(data) {
		if err := f.Truncate(int64(whole)); err != nil {
			return tail{}, err
		}
	}
	t := tail{size: int64(whole)}
	for _, e := range parse(data[:whole]) {
		if e.Kind == KindHead && e.Version > Version {
			return tail{}, ErrNewer
		}
		t.seq, t.at = max(t.seq, e.Seq), max(t.at, e.At)
	}
	return t, nil
}

// parse reads the records of a log, skipping any line that is not one, as one cut off by a crash.
func parse(data []byte) []Event {
	var out []Event
	for line := range bytes.SplitSeq(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var e Event
		if json.Unmarshal(line, &e) != nil || e.Kind == "" {
			continue
		}
		out = append(out, e)
	}
	return out
}

// Read reads every device's log of the video at videoPath into one document. A film with no log reads as an empty document; a log that cannot be read is an error, rather than a document that lacks what it holds.
func Read(videoPath string) (Doc, error) {
	var doc Doc
	for _, path := range Logs(videoPath) {
		data, err := os.ReadFile(path)
		if err != nil {
			return Doc{}, fmt.Errorf("read %s: %w", filepath.Base(path), err)
		}
		device := deviceOf(videoPath, path)
		for _, e := range parse(data) {
			if e.Kind == KindHead && e.Version > Version {
				doc.Newer = true
			}
			e.Device = device
			doc.Events = append(doc.Events, e)
		}
	}
	return doc, nil
}
