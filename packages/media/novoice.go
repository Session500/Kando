package media

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"kando/packages/core"
)

// 弱声段（只有低阈值的 VAD 找得到人声的地方）里听出的句子，要有声音能说出它的证据：这里放判断用的规则。
//
// madeUp reports whether line s, heard over speech only the quiet VAD finds (chunk.quiet), is one no voice there says: it says less than quietMinSyllables, as "-", "ん" or "you", or only names a sound, as "[Music]" or "*sighs*"; it says more than maxSyllableRate syllables a second (syllables), more than any voice says; or it is, but for its marks, one of the lines whisper says over sound with no voice in it at all in this language (noVoice), as "Thank you." The rules look at how much a line says and at what whisper says unasked, never at what a film may say, so a voice in the film saying those very words where the VAD is sure of it, as a television in a scene, is kept.
func madeUp(s core.Segment, noVoice map[string]bool) bool {
	n := syllables(soundLabel.ReplaceAllString(withoutSounds(s.Text), " "))
	if n < quietMinSyllables {
		return true
	}
	if d := s.End - s.Start; d <= 0 || n/d.Seconds() > maxSyllableRate {
		return true
	}
	return noVoice[voiceless(s.Text)]
}

// quietMinSyllables is the least a line heard over the quieter speech must say: whisper writes a single syllable over music as it does over a gasp, so one tells nothing of a voice.
const quietMinSyllables = 2

// soundLabel is a sound named between asterisks, as whisper names one as well as in brackets (sounds): "*music*".
var soundLabel = regexp.MustCompile(`\*[^*]*\*`)

// maxSyllableRate is the most syllables a second a line can say: real speech of nine languages read aloud said 9 at most, and a film's fastest line 13, timed by its words before it is kept up (keepUp).
const maxSyllableRate = 13

// voiceless is text as noVoice compares it: its letters, with the vowel signs and other marks written on them, and digits alone, in lower case.
func voiceless(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, text)
}

// noVoiceMinLetters and noVoiceMinTimes say which of the lines whisper says over noVoiceSounds are what it says where no one speaks: one said over two of them at least, of five letters at least, as it also says a short word that a film says often, such as "はい" or "Okay".
const (
	noVoiceMinLetters = 5
	noVoiceMinTimes   = 2
)

// noVoices is, by noVoiceKey, the lines whisper says where no one speaks (Tools.noVoice).
var noVoices sync.Map

// noVoiceFlight makes the lines of a key be found by one run of whisper-cli at a time.
var noVoiceFlight sync.Mutex

// noVoice is the lines, as voiceless writes them, whisper says in language over sound with no voice in it, by the speech model and the hints Tools hears with: it has whisper-cli hear noVoiceSounds and keeps the lines it says over noVoiceMinTimes of them, of noVoiceMinLetters at least. They are found once for a model, hints and language, and kept beside the model; nil when whisper-cli cannot hear them, as nothing is then told by them.
func (t Tools) noVoice(ctx context.Context, dir, language string) map[string]bool {
	if language == "" || language == "auto" {
		return nil
	}
	key := t.noVoiceKey(language)
	if v, ok := noVoices.Load(key); ok {
		return v.(map[string]bool)
	}
	noVoiceFlight.Lock()
	defer noVoiceFlight.Unlock()
	if v, ok := noVoices.Load(key); ok {
		return v.(map[string]bool)
	}
	saved := filepath.Join(filepath.Dir(t.WhisperModel), "novoice", key+".json")
	if data, err := os.ReadFile(saved); err == nil {
		var lines []string
		if json.Unmarshal(data, &lines) == nil {
			found := setOf(lines)
			noVoices.Store(key, found)
			return found
		}
	}
	lines, err := t.hearNoVoice(ctx, dir, language)
	if err != nil {
		if ctx.Err() == nil {
			logf(ctx, "hear: what whisper says where no one speaks in %s is not known: %v", language, err)
		}
		return nil
	}
	found := setOf(lines)
	noVoices.Store(key, found)
	if data, err := json.Marshal(lines); err == nil && os.MkdirAll(filepath.Dir(saved), 0o755) == nil {
		_ = writeFileAtomic(saved, data)
	}
	return found
}

// noVoiceKey names what noVoice finds for language: by the speech model, as it is on disk, whisper-cli and the hints, which change what whisper says.
func (t Tools) noVoiceKey(language string) string {
	h := fnv.New64a()
	for _, path := range []string{t.WhisperModel, t.Whisper} {
		if info, err := os.Stat(path); err == nil {
			fmt.Fprintf(h, "%s\x00%d\x00%d\x00", path, info.Size(), info.ModTime().UnixNano())
		}
	}
	fmt.Fprintf(h, "%s\x00%d", WhisperPrompt(t.WhisperHints), noVoiceVersion)
	return fmt.Sprintf("%s-%016x", language, h.Sum64())
}

// noVoiceVersion changes when noVoiceSounds or what is kept of them does, so lines found before are found again.
const noVoiceVersion = 1

// hearNoVoice has whisper-cli hear noVoiceSounds in language and returns the lines it says over noVoiceMinTimes of them.
func (t Tools) hearNoVoice(ctx context.Context, dir, language string) ([]string, error) {
	args := append(t.whisperBase(language), "-np", "-oj")
	var paths []string
	for i, pcm := range noVoiceSounds() {
		path := filepath.Join(dir, fmt.Sprintf("novoice-%02d.wav", i))
		if err := writePCM(path, pcm); err != nil {
			return nil, err
		}
		paths = append(paths, path)
		args = append(args, "-f", path)
	}
	ctx, cancel := context.WithTimeout(ctx, noVoiceTimeout)
	defer cancel()
	if _, err := runStage(ctx, t.Whisper, args...); err != nil {
		return nil, err
	}
	times := map[string]int{}
	var order []string
	for _, path := range paths {
		data, err := os.ReadFile(path + ".json")
		if err != nil {
			return nil, err
		}
		heard, err := parseHeard(data)
		if err != nil {
			return nil, err
		}
		for _, l := range heard {
			v := voiceless(l.text)
			if len([]rune(v)) < noVoiceMinLetters {
				continue
			}
			if times[v] == 0 {
				order = append(order, v)
			}
			times[v]++
		}
	}
	lines := []string{}
	for _, v := range order {
		if times[v] >= noVoiceMinTimes {
			lines = append(lines, v)
		}
	}
	return lines, nil
}

func setOf(lines []string) map[string]bool {
	set := make(map[string]bool, len(lines))
	for _, l := range lines {
		set[l] = true
	}
	return set
}

// noVoiceSounds are sounds with no voice in them, as 16 kHz mono 16-bit PCM, the same each time: silence of a few lengths; white, pink and brown noise, soft to loud and short to long; a low rumble, a hiss, wind; a hum, a chord, a held pad, a melody, a drum and clicks: what a film has between its lines.
func noVoiceSounds() [][]byte {
	noise := newNoise(1)
	var out [][]byte
	for _, d := range []float64{0.5, 1, 3, 8, 20} {
		out = append(out, toneOf(d, func(float64) float64 { return 0 }))
	}
	for _, color := range []func() float64{noise.white, noise.pink, noise.brown} {
		for _, a := range []float64{0.003, 0.03, 0.3} {
			out = append(out, toneOf(4, func(float64) float64 { return a * color() }))
		}
	}
	for _, color := range []func() float64{noise.pink, noise.brown} {
		for _, d := range []float64{2, 6, 12, 25} {
			out = append(out, toneOf(d, func(float64) float64 { return 0.02 * color() }))
		}
	}
	sine := func(f, t float64) float64 { return math.Sin(2 * math.Pi * f * t) }
	last := 0.0
	out = append(out,
		toneOf(10, func(float64) float64 { last = 0.9*last + 0.1*noise.brown(); return 0.6 * last }),
		toneOf(5, func(float64) float64 { w := noise.white(); d := w - last; last = w; return 0.05 * d }),
		toneOf(10, func(t float64) float64 { return 0.05 * noise.pink() * (1 - 0.9*(0.5+0.5*sine(0.5, t))) }),
		toneOf(3, func(t float64) float64 { return 0.1 * sine(50, t) }),
		toneOf(6, func(t float64) float64 { return 0.2*sine(220, t) + 0.15*sine(277, t) + 0.15*sine(330, t) }),
		toneOf(15, func(t float64) float64 {
			return 0.05*sine(262, t) + 0.05*sine(330, t) + 0.05*sine(392, t) + 0.025*sine(523, t)
		}),
		toneOf(8, func(t float64) float64 {
			if math.Mod(t, 0.5) >= 0.25 {
				return 0
			}
			return 0.3 * math.Sin(2*math.Pi*(440+200*math.Sin(2*math.Pi*0.5*t))*t)
		}),
		toneOf(10, func(t float64) float64 { return 0.3 * sine(110, t) * math.Exp(-3*math.Mod(t, 1)) }),
		toneOf(6, func(t float64) float64 {
			if math.Mod(t, 0.7) < 0.01 {
				return 0.8
			}
			return 0
		}),
	)
	return out
}

// toneOf is d seconds of the sound sample says, at each time from the start, as PCM.
func toneOf(d float64, sample func(t float64) float64) []byte {
	n := int(d * 16000)
	out := make([]byte, 2*n)
	for i := range n {
		v := max(-1, min(1, sample(float64(i)/16000)))
		binary.LittleEndian.PutUint16(out[2*i:], uint16(int16(v*math.MaxInt16)))
	}
	return out
}

// noise makes white, pink and brown noise from a seed, the same each time.
type noise struct {
	state            uint64
	b0, b1, b2, walk float64
}

func newNoise(seed uint64) *noise { return &noise{state: seed} }

// white is a sample of white noise between -1 and 1.
func (n *noise) white() float64 {
	n.state ^= n.state << 13
	n.state ^= n.state >> 7
	n.state ^= n.state << 17
	return float64(n.state>>11)/float64(1<<52)*2 - 1
}

// pink is a sample of pink noise, white filtered to fall 3 dB an octave (Paul Kellet's economy filter).
func (n *noise) pink() float64 {
	w := n.white()
	n.b0 = 0.99765*n.b0 + w*0.0990460
	n.b1 = 0.96300*n.b1 + w*0.2965164
	n.b2 = 0.57000*n.b2 + w*1.0526913
	return (n.b0 + n.b1 + n.b2 + w*0.1848) * 0.25
}

// brown is a sample of brown noise, white summed, leaking back to 0.
func (n *noise) brown() float64 {
	n.walk = max(-1, min(1, 0.998*n.walk+0.05*n.white()))
	return n.walk
}

// noVoiceTimeout bounds how long whisper-cli may take to hear noVoiceSounds, about four minutes of sound, which it hears in ten seconds or so.
const noVoiceTimeout = 5 * time.Minute
