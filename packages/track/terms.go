package track

import (
	"slices"
)

// MaxTerms is how many terms a film has at most: the corrections found once it has that many are not made a term, so a film whose proofreading finds ever more corrections does not have its lines made again without end.
const MaxTerms = 200

// Term is a correction made in two lines or more of a film and never another way: its words From, with the word before and the word after them, put as To, each holding its words with a NUL between them.
type Term struct {
	From, To string
}

// Terms are the terms of generation gen, sorted by From: the corrections its proofreading made (KindFix) in two lines or more in all, and never another way, the first MaxTerms of them to become one. A term once made stays one: a later correction made another way does not undo it, so the proofreading cannot turn a term back and forth.
func (d Doc) Terms(gen string) []Term {
	type seen struct {
		to     string
		lines  int
		mixed  bool
		termAt int
	}
	fixes := map[string]*seen{}
	var order []string
	made := 0
	for _, e := range d.Events {
		if e.Kind != KindFix || e.Gen != gen || e.Text == "" {
			continue
		}
		f := fixes[e.Text]
		if f == nil {
			f = &seen{to: e.To, termAt: -1}
			fixes[e.Text] = f
		}
		if f.termAt >= 0 {
			continue
		}
		if f.to != e.To {
			f.mixed = true
		}
		f.lines += e.Count
		if !f.mixed && f.lines >= 2 && made < MaxTerms {
			f.termAt = made
			made++
			order = append(order, e.Text)
		}
	}
	out := make([]Term, 0, len(order))
	for _, from := range order {
		out = append(out, Term{From: from, To: fixes[from].to})
	}
	slices.SortFunc(out, func(a, b Term) int {
		switch {
		case a.From < b.From:
			return -1
		case a.From > b.From:
			return 1
		}
		return 0
	})
	return out
}
