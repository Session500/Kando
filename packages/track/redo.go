package track

import (
	"cmp"
	"slices"
)

// Redoing is the film's lines as they show while generation gen makes the film again, which the film shows only once it is done (KindDone): each cell gen has drafted shows gen's lines, and every other cell the lines of the generation the film shows (Shown), so a film translated or processed again while it is watched shows its new lines where they are made, nearest the playhead first, and the film as it was elsewhere. A line belongs to the cell its middle falls in. A line the film shows that a line gen made in another cell covers half of or more goes, so a line gen cuts differently does not show twice, as two runs of the speech recognition keep the lines of their own cells; one covered less shows beside it, so none goes missing. A cell gen did not draft, as one whose draft was given up, shows the film as it was. keep is false for a film being translated into another language than the one it shows: the lines not made again show no translation, as lines not translated yet, rather than one in a language the film no longer goes into. gen "", the one the film shows, or one done shows the film's lines (View).
func (d Doc) Redoing(gen string, keep bool) []Line {
	shown := d.Shown()
	if gen == "" || gen == shown || d.Done(gen) {
		return d.View()
	}
	drafted := map[int]bool{}
	for _, e := range d.Events {
		if e.Kind == KindUnit && e.Gen == gen && Work(e.Source) == WorkDraft {
			drafted[unitOf(e).Cell] = true
		}
	}
	cellOf := func(l Line) int { return int((l.Start + l.End) / 2 / Cell) }
	made := map[int][]Line{}
	var out []Line
	for _, l := range d.Lines(gen) {
		if c := cellOf(l); drafted[c] {
			made[c] = append(made[c], l)
			out = append(out, l)
		}
	}
	// covered reports whether a line gen made next to l's cell covers half of l or more: l's own cell is not drafted, and a line reaches into the cells beside its own.
	covered := func(l Line) bool {
		c := cellOf(l)
		for _, m := range slices.Concat(made[c-1], made[c+1]) {
			if over := min(l.End, m.End) - max(l.Start, m.Start); over > 0 && 2*over >= l.End-l.Start {
				return true
			}
		}
		return false
	}
	for _, l := range d.Lines(shown) {
		if drafted[cellOf(l)] || covered(l) {
			continue
		}
		if !keep {
			l.Translation = Text{}
		}
		out = append(out, l)
	}
	slices.SortFunc(out, func(a, b Line) int {
		return cmp.Or(cmp.Compare(a.Start, b.Start), cmp.Compare(a.End, b.End), cmp.Compare(a.ID, b.ID))
	})
	return out
}
