package track

import (
	"slices"
	"testing"
)

func fix(gen, from, to string, count int) Event {
	return Event{Kind: KindFix, Gen: gen, Text: from, To: to, Count: count}
}

func TestAFixMadeInTwoLinesIsATerm(t *testing.T) {
	doc := Doc{}.Plus("mac",
		fix("g", "网罗", "网络", 1),
		fix("g", "倒一", "退一", 1),
		fix("other", "倒一", "退一", 1),
	)
	if got := doc.Terms("g"); len(got) != 0 {
		t.Fatalf("Terms = %v, want none made in one line", got)
	}
	// Made once more in a later batch, the first is a term; the second was made once in this generation only.
	doc = doc.Plus("mac", fix("g", "网罗", "网络", 1))
	if got := doc.Terms("g"); !slices.Equal(got, []Term{{From: "网罗", To: "网络"}}) {
		t.Fatalf("Terms = %v", got)
	}
}

func TestATermIsNotTurnedBack(t *testing.T) {
	doc := Doc{}.Plus("mac", fix("g", "a", "b", 2), fix("g", "a", "c", 5), fix("g", "x", "y", 1), fix("g", "x", "z", 1))
	// a became b first and stays so; x was made two ways before it was a term, so it never is one.
	if got := doc.Terms("g"); !slices.Equal(got, []Term{{From: "a", To: "b"}}) {
		t.Fatalf("Terms = %v", got)
	}
}

func TestAFilmHasAtMostMaxTerms(t *testing.T) {
	var events []Event
	for i := range MaxTerms + 5 {
		events = append(events, fix("g", string(rune('a'+i%26))+string(rune('A'+i/26)), "x", 2))
	}
	if got := len(Doc{}.Plus("mac", events...).Terms("g")); got != MaxTerms {
		t.Fatalf("%d terms, want %d", got, MaxTerms)
	}
}
