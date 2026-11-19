// Package track is a film's subtitles as one document: its lines, each with the revisions its original and its translation went through, which every other form of them, the subtitle files beside the video included, is projected from.
//
// The document is kept beside the video as one log per device (LogPath), which only that device appends to, so a library folder shared between computers or synced by a sync service never has two writers on one file; reading the film merges every device's log (Read). A revision never replaces another in the log: what a line shows is decided when the log is read (View), the revision of the highest layer winning, and of two in the same layer the later.
package track

import "time"

// Version is the format of the logs this package writes. A log whose head names a later one was written by a newer Kando: it is read as far as it can be, and not written to (Doc.Newer).
const Version = 1

// Kind is what an event records.
type Kind string

const (
	// KindHead opens a device's log: the format it is written in.
	KindHead Kind = "head"
	// KindGen starts a generation of lines (Gen): the lines one making of the film's subtitles heard, as a first run or a redo.
	KindGen Kind = "gen"
	// KindDone says generation Gen holds every line of the film, so the view may take it over an earlier one.
	KindDone Kind = "done"
	// KindLine is a line of generation Gen, Line its id, shown from Start to End.
	KindLine Kind = "line"
	// KindRev is a revision of Side of Line: its text as Layer wrote it, from Source, made from the inputs Key names.
	KindRev Kind = "rev"
	// KindGone takes Line out of its generation, as a line found to be no speech.
	KindGone Kind = "gone"
	// KindFile records the subtitle file the document last took in or wrote out: its name (Source) and the hash of what it held (Key). A file that holds anything else was written by something other than the document, as an older Kando or the user's editor, and is taken in again (media.ReadTrack).
	KindFile Kind = "file"
	// KindUnit records that a unit of work was made (Unit.Made): Source is its Work, Gen its generation, Start and End its cell, Key its inputs. What it wrote comes before it in the log, so a unit recorded made has all it wrote.
	KindUnit Kind = "unit"
	// KindAttempt records a failed attempt at a unit (Unit.Failed), with why in Text; a unit tried Attempts times is given up.
	KindAttempt Kind = "attempt"
)

// Side is which text of a line a revision is of.
type Side string

const (
	Original    Side = "o"
	Translation Side = "t"
)

// Layer is who wrote a revision. A line shows, of each side, the revision of the highest layer.
type Layer int8

const (
	// LayerHeard is the speech recognition's line, or the site's subtitles.
	LayerHeard Layer = 1
	// LayerDraft is the machine's correction of what was heard and its translation made while the film is made, before the film is read through as a whole.
	LayerDraft Layer = 2
	// LayerFinal is the final proofread's, which read the film as a whole, and subtitles brought in from files made before the document (Import).
	LayerFinal Layer = 3
	// LayerUser is the user's own.
	LayerUser Layer = 4
)

// Event is one record of a log. Device is the device whose log holds it, taken from the log's name rather than written in every record; Seq counts the device's records from 1 and At is when it wrote them, in milliseconds, never less than the record before (Append), so the records of one device keep their order whatever its clock did.
type Event struct {
	Kind   Kind   `json:"k"`
	Seq    int64  `json:"n"`
	At     int64  `json:"t"`
	Device string `json:"-"`

	Version int    `json:"v,omitempty"`
	Gen     string `json:"g,omitempty"`
	Line    string `json:"l,omitempty"`
	Start   int64  `json:"s,omitempty"`
	End     int64  `json:"e,omitempty"`
	Side    Side   `json:"side,omitempty"`
	Layer   Layer  `json:"layer,omitempty"`
	Text    string `json:"x,omitempty"`
	Source  string `json:"src,omitempty"`
	Key     string `json:"key,omitempty"`
}

// Span is when the line shows.
func (e Event) Span() (time.Duration, time.Duration) {
	return time.Duration(e.Start) * time.Millisecond, time.Duration(e.End) * time.Millisecond
}

// after reports whether e was written after o: the later time, and of two at the same time the device whose id sorts last, as study.Stamp decides, so every device that reads the same logs picks the same one.
func (e Event) after(o Event) bool {
	if e.At != o.At {
		return e.At > o.At
	}
	if e.Device != o.Device {
		return e.Device > o.Device
	}
	return e.Seq > o.Seq
}
