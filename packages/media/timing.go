package media

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"kando/packages/core"
)

// How a line heard in a chunk is timed by its words: it starts wordLead before its first word and ends wordTail after its last, never across the edge of the speech or pause its word is in, and stays up at least lineShortest unless the next line starts sooner. A line longer than lineLongest is cut in the longest pause of at least cutPause between its words, again and again, as a subtitle shown longer than lineLongest is no longer read as one.
const (
	wordLead     = 300 * time.Millisecond
	wordTail     = 600 * time.Millisecond
	lineShortest = time.Second
	lineLongest  = 7 * time.Second
	cutPause     = 500 * time.Millisecond
	// lineShortestShared is how long a line stays up when the next starts at once.
	lineShortestShared = 200 * time.Millisecond
)

// The fallback for a line whose words carry no times (startInSpeech): a start more than startInSpeech into a stretch of speech moves to the next stretch when that starts at least speechBeforeEnd before the line ends.
const (
	startInSpeech   = 300 * time.Millisecond
	speechBeforeEnd = 500 * time.Millisecond
)

// sentenceEnds are the marks a sentence ends with, where a line whisper heard as one is cut when a pause, however short, follows.
const sentenceEnds = "。？！?!"

// decodedLine is a line whisper-cli heard in a file, as its JSON says, in the file's time.
type decodedLine struct {
	from, to time.Duration
	text     string
	tokens   []decodedToken
	// until is when the last word of the last of the same line said again and again is, in the file's time, once they are made one (collapseRepeats); 0 when the line was said once.
	until time.Duration
}

// decodedToken is a piece of a line's text as whisper read it, with the time DTW puts it at when timed.
type decodedToken struct {
	text  string
	at    time.Duration
	timed bool
}

// whisperFullOutput is the file whisper-cli writes with -ojf: offsets are milliseconds, t_dtw centiseconds or -1 when the token was not timed.
type whisperFullOutput struct {
	Transcription []struct {
		Offsets struct {
			From int64 `json:"from"`
			To   int64 `json:"to"`
		} `json:"offsets"`
		Text   string `json:"text"`
		Tokens []struct {
			Text string `json:"text"`
			DTW  *int64 `json:"t_dtw"`
		} `json:"tokens"`
	} `json:"transcription"`
}

// parseHeard reads the file whisper-cli writes with -oj or -ojf. whisper escapes only quotes and backslashes in its strings, so control characters are dropped first; between the values they are spaces JSON can do without.
func parseHeard(data []byte) ([]decodedLine, error) {
	data = bytes.Map(func(r rune) rune {
		if r < 0x20 {
			return -1
		}
		return r
	}, data)
	var out whisperFullOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode whisper output: %w", err)
	}
	lines := make([]decodedLine, 0, len(out.Transcription))
	for _, t := range out.Transcription {
		text := strings.TrimSpace(t.Text)
		if text == "" {
			continue
		}
		l := decodedLine{from: time.Duration(t.Offsets.From) * time.Millisecond, to: time.Duration(t.Offsets.To) * time.Millisecond, text: text}
		for _, k := range t.Tokens {
			tok := decodedToken{text: k.Text}
			if k.DTW != nil && *k.DTW >= 0 {
				tok.at, tok.timed = time.Duration(*k.DTW)*10*time.Millisecond, true
			}
			l.tokens = append(l.tokens, tok)
		}
		lines = append(lines, l)
	}
	return lines, nil
}

// special reports whether a token is one of whisper's own, such as a timestamp, rather than text.
func (k decodedToken) special() bool { return strings.HasPrefix(k.text, "[_") }

// word reports whether a token is a timed piece of a word: not whisper's own, nor only marks and spaces.
func (k decodedToken) word() bool {
	return k.timed && !k.special() && strings.IndexFunc(k.text, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }) >= 0
}

// timeLines is the lines heard in chunk c, in the time of the sound, each timed by its words when DTW timed them (wordTimes), else by whisper's own times (placeLine).
func timeLines(heard []decodedLine, c chunk, wordTimes bool) []core.Segment {
	var out []core.Segment
	for _, l := range collapseRepeats(heard) {
		var words []int
		if wordTimes {
			for i, k := range l.tokens {
				if k.word() {
					words = append(words, i)
				}
			}
		}
		if len(words) == 0 {
			out = append(out, placeLine(core.Segment{Start: l.from, End: l.to, Text: l.text}, c))
			continue
		}
		if !textOfTokens(l) {
			// The tokens do not spell the line, so it is not cut: it is timed as a whole by its words.
			out = append(out, timeWords(l, words, [][]int{words}, c))
			continue
		}
		pieces := cutAtSentenceEnds(l, words, c)
		for _, p := range pieces {
			for _, q := range cutLong(l, p, c) {
				out = append(out, timeWords(l, q, pieces, c))
			}
		}
	}
	slices.SortStableFunc(out, func(a, b core.Segment) int { return cmp.Compare(a.Start, b.Start) })
	return keepUp(out, c)
}

// collapseRepeats is lines with each run of the same line said again and again made one: within a chunk whisper may still say a line over a few times, some of them for a split second, where it was said once.
func collapseRepeats(lines []decodedLine) []decodedLine {
	var out []decodedLine
	for _, l := range lines {
		if n := len(out); n > 0 && out[n-1].text == l.text {
			last := &out[n-1]
			last.to = max(last.to, l.to)
			for _, k := range l.tokens {
				if k.word() {
					last.until = max(last.until, k.at)
				}
			}
			continue
		}
		out = append(out, l)
	}
	return out
}

// textOfTokens reports whether line l's tokens, but whisper's own, spell its text, so the line can be cut between them: a token that holds part of a character, as an older whisper-cli may write, does not.
func textOfTokens(l decodedLine) bool {
	var b strings.Builder
	for _, k := range l.tokens {
		if !k.special() {
			b.WriteString(k.text)
		}
	}
	s := b.String()
	return strings.TrimSpace(s) == l.text && utf8.ValidString(s) && !strings.ContainsRune(s, utf8.RuneError)
}

// cutAtSentenceEnds is the words of line l, by index into its tokens, split where a sentence ends and the speech pauses, however briefly, before the next word: whisper may hear two people's lines in a row as one.
func cutAtSentenceEnds(l decodedLine, words []int, c chunk) [][]int {
	out := [][]int{{words[0]}}
	for n := 1; n < len(words); n++ {
		p, k := words[n-1], words[n]
		ends := false
		for _, t := range l.tokens[p:k] {
			if s := strings.TrimSpace(t.text); s != "" && strings.ContainsRune(sentenceEnds, lastRune(s)) {
				ends = true
			}
		}
		if ends && edgeBetween(c, c.from+l.tokens[p].at, c.from+l.tokens[k].at) {
			out = append(out, []int{k})
			continue
		}
		out[len(out)-1] = append(out[len(out)-1], k)
	}
	return out
}

// cutLong is the words of a piece of line l cut in the longest pause between them, again and again, until no piece is longer than lineLongest, or no pause of cutPause is left to cut in.
func cutLong(l decodedLine, words []int, c chunk) [][]int {
	first, last := l.tokens[words[0]].at, l.tokens[words[len(words)-1]].at
	if len(words) < 2 || last-first+wordLead+wordTail <= lineLongest {
		return [][]int{words}
	}
	best, longest := 0, time.Duration(-1)
	for n := 1; n < len(words); n++ {
		if p := pauseBetween(c, c.from+l.tokens[words[n-1]].at, c.from+l.tokens[words[n]].at); p > longest {
			best, longest = n, p
		}
	}
	if longest < cutPause {
		return [][]int{words}
	}
	return append(cutLong(l, words[:best], c), cutLong(l, words[best:], c)...)
}

// timeWords is the piece of line l whose words are words, of the line cut into pieces: from wordLead before its first word to wordTail after its last, within the speech or pause each is in, saying the line's text, or once the line is cut the tokens from its first word up to the next piece's.
func timeWords(l decodedLine, words []int, pieces [][]int, c chunk) core.Segment {
	first, last := c.from+l.tokens[words[0]].at, c.from+l.tokens[words[len(words)-1]].at
	if words[len(words)-1] == lastWord(pieces) && l.until > 0 {
		last = max(last, c.from+l.until)
	}
	s := core.Segment{
		Start: max(first-wordLead, edgeBefore(c, first)),
		End:   min(last+wordTail, edgeAfter(c, last)),
		Text:  l.text,
	}
	if len(pieces) > 1 || len(words) < countWords(pieces) {
		s.Text = textFrom(l, words[0], words[len(words)-1], pieces)
	}
	return s
}

// textFrom is the text of line l from the token of word first, or the line's start for its first word, up to the word after last, which starts the next piece, or the line's end.
func textFrom(l decodedLine, first, last int, pieces [][]int) string {
	from := first
	if first == pieces[0][0] {
		from = 0
	}
	to := len(l.tokens)
	for k := last + 1; k < len(l.tokens); k++ {
		if l.tokens[k].word() {
			to = k
			break
		}
	}
	var b strings.Builder
	for _, k := range l.tokens[from:to] {
		if !k.special() {
			b.WriteString(k.text)
		}
	}
	return strings.TrimSpace(b.String())
}

func lastWord(pieces [][]int) int {
	p := pieces[len(pieces)-1]
	return p[len(p)-1]
}

func countWords(pieces [][]int) int {
	n := 0
	for _, p := range pieces {
		n += len(p)
	}
	return n
}

func lastRune(s string) rune {
	r, _ := utf8.DecodeLastRuneInString(s)
	return r
}

// edgeBefore is the last edge of speech at or before at, in chunk c's time: the start of the stretch of speech at is in, the end of the one before the pause it is in, else the chunk's start.
func edgeBefore(c chunk, at time.Duration) time.Duration {
	edge := c.from
	for _, sp := range c.speech {
		if sp.from > at {
			break
		}
		edge = sp.from
		if sp.to < at {
			edge = sp.to
		}
	}
	return edge
}

// edgeAfter is the first edge of speech at or after at: the end of the stretch of speech at is in, the start of the one after the pause it is in, else the chunk's end.
func edgeAfter(c chunk, at time.Duration) time.Duration {
	for _, sp := range c.speech {
		if sp.to >= at {
			if sp.from > at {
				return sp.from
			}
			return sp.to
		}
	}
	return c.to
}

// edgeBetween reports whether a stretch of speech ends between a and b, as at a pause, however short.
func edgeBetween(c chunk, a, b time.Duration) bool {
	for _, sp := range c.speech {
		if sp.to > a && sp.to <= b {
			return true
		}
	}
	return false
}

// pauseBetween is how long the speech pauses between a and b.
func pauseBetween(c chunk, a, b time.Duration) time.Duration {
	var pause time.Duration
	for i := 0; i+1 < len(c.speech); i++ {
		from, to := max(a, c.speech[i].to), min(b, c.speech[i+1].from)
		if to > from {
			pause += to - from
		}
	}
	return pause
}

// placeLine is line s, heard in the time of chunk c's file, in the time of the sound, by whisper's own times: within c, and starting where its speech does when whisper starts it in a pause before it, or well into the speech before it, as whisper may start a line where the one before it ended.
func placeLine(s core.Segment, c chunk) core.Segment {
	s.Start = min(c.from+s.Start, c.to)
	s.End = min(c.from+s.End, c.to)
	for i, sp := range c.speech {
		if s.Start >= sp.from && s.Start < sp.to {
			if s.Start-sp.from > startInSpeech && i+1 < len(c.speech) && c.speech[i+1].from < s.End-speechBeforeEnd {
				s.Start = c.speech[i+1].from
			}
			break
		}
		if sp.from > s.Start {
			if sp.from < s.End {
				s.Start = sp.from
			}
			break
		}
	}
	return s
}

// keepUp is the lines of chunk c in order, each up at least lineShortest unless the next starts sooner, and none past the next's start or the chunk's end.
func keepUp(lines []core.Segment, c chunk) []core.Segment {
	for i := range lines {
		s := &lines[i]
		s.End = min(max(s.End, s.Start+lineShortest), c.to)
		if i+1 < len(lines) && s.End > lines[i+1].Start {
			s.End = max(lines[i+1].Start, s.Start+lineShortestShared)
		}
	}
	return lines
}

// dtwPreset is the name whisper-cli's -dtw knows the model at path by, from the sizes its header gives, or "" when they name no model DTW is known for: large-v1 and large-v2 have the same sizes and different alignment heads, so neither is named.
func dtwPreset(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	var h struct {
		Magic                                                [4]byte
		Vocab, AudioCtx, AudioState, AudioHead, AudioLayer   int32
		TextCtx, TextState, TextHead, TextLayer, Mels, Ftype int32
	}
	if err := binary.Read(io.LimitReader(f, 48), binary.LittleEndian, &h); err != nil || string(h.Magic[:]) != "lmgg" {
		return ""
	}
	english := h.Vocab == 51864
	name := ""
	switch {
	case h.TextState == 384 && h.TextLayer == 4:
		name = "tiny"
	case h.TextState == 512 && h.TextLayer == 6:
		name = "base"
	case h.TextState == 768 && h.TextLayer == 12:
		name = "small"
	case h.TextState == 1024 && h.TextLayer == 24:
		name = "medium"
	case h.TextState == 1280 && h.TextLayer == 32 && h.Mels == 128:
		return "large.v3"
	case h.TextState == 1280 && h.TextLayer == 4 && h.Mels == 128:
		return "large.v3.turbo"
	default:
		return ""
	}
	if english {
		name += ".en"
	}
	return name
}
