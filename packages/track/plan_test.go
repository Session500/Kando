package track

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

// making is a film of length made in generation "g", watched from playhead, heard in Japanese and translated into Chinese.
func making(length, playhead time.Duration) Making {
	return Making{Gen: "g", Length: length, Playhead: playhead, Hearing: "ja", Language: "zh"}
}

// hearCell is the records of hearing unit u's cell: one line for each of texts, in the middle of the cell, then the unit made.
func hearCell(u Unit, texts ...string) []Event {
	start, _ := u.Span()
	var out []Event
	for i, text := range texts {
		id := fmt.Sprintf("c%d-%d", u.Cell, i)
		at := start + time.Duration(i+1)*time.Second
		out = append(out,
			Event{Kind: KindLine, Gen: u.Gen, Line: id, Start: at.Milliseconds(), End: (at + 500*time.Millisecond).Milliseconds()},
			Event{Kind: KindRev, Line: id, Side: Original, Layer: LayerHeard, Text: text},
		)
	}
	return append(out, u.Made())
}

func cells(units []Unit) []int {
	out := make([]int, len(units))
	for i, u := range units {
		out[i] = u.Cell
	}
	return out
}

func TestPlanStartsAtThePlayhead(t *testing.T) {
	p := Doc{}.Plan(making(200*time.Second, 95*time.Second))
	if p.Cells != 7 {
		t.Fatalf("Cells = %d, want 7", p.Cells)
	}
	// The window starts two seconds before the playhead, in cell 3 (90–120 s), and wraps to the start once the end is planned.
	if got := cells(p.Hear); !slices.Equal(got, []int{3, 4, 5, 6, 0, 1, 2}) {
		t.Fatalf("Hear = %v", got)
	}
	// Two seconds into a cell, the line just said belongs to the cell before.
	if got := cells(Doc{}.Plan(making(200*time.Second, 61*time.Second)).Hear)[0]; got != 1 {
		t.Fatalf("first cell = %d, want the one the line just said is in", got)
	}
	if p.Done() || len(p.Draft) != 0 {
		t.Fatalf("a film not heard yet is %+v", p)
	}
}

func TestPlanDraftsAHeardCell(t *testing.T) {
	m := making(60*time.Second, 0)
	doc := Doc{}
	doc = doc.Plus("mac", hearCell(Doc{}.Plan(m).Hear[0], "おはよう", "またね")...)
	p := doc.Plan(m)
	if got := cells(p.Hear); !slices.Equal(got, []int{1}) {
		t.Fatalf("Hear = %v", got)
	}
	if got := cells(p.Draft); !slices.Equal(got, []int{0}) {
		t.Fatalf("Draft = %v", got)
	}
	// The second cell holds no speech: once heard it is done, with nothing to draft.
	doc = doc.Plus("mac", hearCell(p.Hear[0])...)
	doc = doc.Plus("mac", p.Draft[0].Made())
	if p := doc.Plan(m); !p.Done() || p.Heard != 2 || p.Drafted != 2 {
		t.Fatalf("plan = %+v, want done", p)
	}
}

func TestPlanDoesNotDraftAFilmNotTranslated(t *testing.T) {
	m := making(30*time.Second, 0)
	m.Language = ""
	doc := Doc{}.Plus("mac", hearCell(Doc{}.Plan(m).Hear[0], "hello")...)
	if p := doc.Plan(m); !p.Done() {
		t.Fatalf("plan = %+v, want done once heard", p)
	}
}

func TestPlanGivesUpAUnitTriedThreeTimes(t *testing.T) {
	m := making(30*time.Second, 0)
	doc := Doc{}
	u := doc.Plan(m).Hear[0]
	for i := range Attempts {
		if p := doc.Plan(m); len(p.Hear) != 1 {
			t.Fatalf("after %d failures Hear = %v", i, p.Hear)
		}
		doc = doc.Plus("mac", u.Failed("whisper crashed"))
	}
	p := doc.Plan(m)
	if len(p.Hear) != 0 || !slices.Equal(p.GivenUp, []Unit{u}) || !p.Done() {
		t.Fatalf("plan = %+v, want the cell given up", p)
	}
	// Heard another way, as in the language the user named, the cell is tried again.
	m.Hearing = "en"
	if p := doc.Plan(m); len(p.Hear) != 1 || p.Done() {
		t.Fatalf("plan = %+v, want the cell heard again", p)
	}
}

func TestADraftThatCorrectsItsLinesIsNotNeededAgain(t *testing.T) {
	m := making(30*time.Second, 0)
	doc := Doc{}.Plus("mac", hearCell(Doc{}.Plan(m).Hear[0], "機関")...)
	draft := doc.Plan(m).Draft[0]
	// The draft corrects what was heard and translates it.
	doc = doc.Plus("mac",
		Event{Kind: KindRev, Line: "c0-0", Side: Original, Layer: LayerDraft, Text: "期間"},
		Event{Kind: KindRev, Line: "c0-0", Side: Translation, Layer: LayerDraft, Text: "期间"},
		draft.Made(),
	)
	if p := doc.Plan(m); !p.Done() {
		t.Fatalf("plan = %+v, want done", p)
	}
	// A line taken out of the cell changes what its draft is made from.
	doc = doc.Plus("mac", Event{Kind: KindLine, Gen: "g", Line: "late", Start: 5000, End: 6000}, Event{Kind: KindRev, Line: "late", Side: Original, Layer: LayerHeard, Text: "x"})
	if p := doc.Plan(m); len(p.Draft) != 1 || p.Draft[0].Key == draft.Key {
		t.Fatalf("plan = %+v, want the cell drafted again with its new line", p)
	}
}

func TestPlanOfAFilmOfUnknownLength(t *testing.T) {
	if p := (Doc{}).Plan(making(0, 0)); p.Done() || len(p.Hear) != 0 {
		t.Fatalf("plan = %+v, want nothing planned and not done", p)
	}
}

// TestPlanAlwaysEnds runs a film to its end as a scheduler would, the work failing at random and the playhead jumping, and checks that every run ends with nothing left to do, each unit tried at most Attempts times: no cell is waited on for ever, and none is given out again and again.
func TestPlanAlwaysEnds(t *testing.T) {
	for seed := range uint64(200) {
		r := rand.New(rand.NewPCG(seed, seed*7+1))
		length := time.Duration(1+r.IntN(400)) * time.Second
		m := making(length, time.Duration(r.Int64N(int64(length))))
		if r.IntN(4) == 0 {
			m.Language = ""
		}
		if r.IntN(2) == 0 {
			m.Final = func([]string) string { return "terms" }
		}
		doc := Doc{}
		given := map[Unit]int{}
		limit := 3 * int((length+Cell-1)/Cell) * Attempts
		steps := 0
		for p := doc.Plan(m); !p.Done(); p = doc.Plan(m) {
			if steps++; steps > limit {
				t.Fatalf("seed %d: not done after %d units: %+v", seed, limit, p)
			}
			if r.IntN(5) == 0 {
				m.Playhead = time.Duration(r.Int64N(int64(length)))
			}
			var u Unit
			switch {
			case len(p.Final) > 0 && r.IntN(3) == 0:
				u = p.Final[0]
			case len(p.Draft) > 0 && (len(p.Hear) == 0 || r.IntN(2) == 0):
				u = p.Draft[0]
			case len(p.Hear) > 0:
				u = p.Hear[0]
			case len(p.Final) > 0:
				u = p.Final[0]
			default:
				t.Fatalf("seed %d: nothing to do yet not done: %+v", seed, p)
			}
			if given[u]++; given[u] > Attempts {
				t.Fatalf("seed %d: %+v given out %d times", seed, u, given[u])
			}
			if r.IntN(3) == 0 {
				doc = doc.Plus("mac", u.Failed("failed"))
				continue
			}
			if u.Work != WorkHear {
				doc = doc.Plus("mac", u.Made())
				continue
			}
			texts := make([]string, r.IntN(3))
			for i := range texts {
				texts[i] = fmt.Sprintf("line %d", r.IntN(1000))
			}
			doc = doc.Plus("mac", hearCell(u, texts...)...)
		}
	}
}

func TestPlanProofreadsABlockOnceItIsDrafted(t *testing.T) {
	m := making(330*time.Second, 0)
	key := "terms:none"
	m.Final = func([]string) string { return key }
	doc := Doc{}
	run := func() Plan {
		for {
			p := doc.Plan(m)
			switch {
			case len(p.Hear) > 0:
				doc = doc.Plus("mac", hearCell(p.Hear[0], "line")...)
			case len(p.Draft) > 0:
				doc = doc.Plus("mac", p.Draft[0].Made())
			default:
				return p
			}
		}
	}
	p := run()
	// 5:30 is two blocks: the first of ten cells, the second of one.
	if p.Blocks != 2 || len(p.Final) != 2 || p.Final[0].Cell != 0 || p.Final[1].Cell != Block || p.Done() {
		t.Fatalf("plan = %+v", p)
	}
	doc = doc.Plus("mac", p.Final[0].Made(), p.Final[1].Made())
	if p := doc.Plan(m); !p.Done() || !p.Finished[0] || !p.Finished[1] {
		t.Fatalf("plan = %+v, want done", p)
	}
	// A term found in the first block makes it proofread again; the second, without it, stays final.
	m.Final = func(heard []string) string {
		if len(heard) > 3 {
			return "terms:foo"
		}
		return key
	}
	if p := doc.Plan(m); len(p.Final) != 1 || p.Final[0].Cell != 0 || p.Finished[0] || !p.Finished[1] {
		t.Fatalf("plan = %+v, want the first block proofread again", p)
	}
}

func TestABlockIsNotProofreadBeforeItsCellsAreDrafted(t *testing.T) {
	m := making(60*time.Second, 0)
	m.Final = func([]string) string { return "" }
	doc := Doc{}.Plus("mac", hearCell(Doc{}.Plan(m).Hear[0], "line")...)
	if p := doc.Plan(m); len(p.Final) != 0 {
		t.Fatalf("Final = %v before the block is drafted", p.Final)
	}
}

func TestFinalLinesAreThoseOfAFinishedBlock(t *testing.T) {
	m := making(330*time.Second, 0)
	m.Final = func([]string) string { return "" }
	doc := Doc{}
	for {
		p := doc.Plan(m)
		if len(p.Hear) > 0 {
			doc = doc.Plus("mac", hearCell(p.Hear[0], "line")...)
			continue
		}
		if len(p.Draft) > 0 {
			doc = doc.Plus("mac", p.Draft[0].Made())
			continue
		}
		// Only the second block is proofread.
		doc = doc.Plus("mac", p.Final[1].Made())
		break
	}
	final := doc.Final("g", m.Language, m.Final)
	for _, l := range doc.Lines("g") {
		if want := l.Start >= Block*Cell; final[l.ID] != want {
			t.Fatalf("line at %v final %v, want %v", l.Start, final[l.ID], want)
		}
	}
	// A line the user wrote is final wherever it is.
	first := doc.Lines("g")[0].ID
	doc = doc.Plus("mac", Event{Kind: KindRev, Line: first, Side: Translation, Layer: LayerUser, Text: "mine"})
	if !doc.Final("g", m.Language, m.Final)[first] {
		t.Fatal("a line the user wrote is not final")
	}
	// Without a final proofread, a line is final only when written so.
	if got := doc.Final("g", m.Language, nil); len(got) != 1 || !got[first] {
		t.Fatalf("final without a proofread = %v", got)
	}
}
