package media

import (
	"strings"
	"unicode"
)

// syllables is about how many syllables saying text takes, in any language, from its script alone: a character for a Chinese character, a Hangul block, a kana that is not small or a long mark, or a digit; a vowel group for a word written in an alphabet (Latin, Cyrillic, Greek), at least one a word; for a script that writes a consonant with its vowel marked on it, a consonant, less one joined to the next by a virama (Devanagari and the Indic scripts), or one of 1.6 (Thai, Lao, Khmer, Burmese, written without spaces), or one letter of two for a script that leaves most vowels out (Hebrew, Arabic). A script it does not know counts a syllable a word. It is an estimate for telling a line no voice could say in the time it is shown (madeUp), not a count of a language's syllables.
func syllables(text string) float64 {
	var n, southeast, indic, abjad float64
	var word []rune
	flush := func() {
		if len(word) == 0 {
			return
		}
		groups, inVowel := 0, false
		for _, r := range word {
			if unicode.Is(unicode.Mn, r) {
				continue
			}
			v := isVowel(unicode.ToLower(r))
			if v && !inVowel {
				groups++
			}
			inVowel = v
		}
		n += float64(max(1, groups))
		word = word[:0]
	}
	for _, r := range text {
		switch {
		case unicode.In(r, unicode.Latin, unicode.Cyrillic, unicode.Greek) && unicode.IsLetter(r):
			word = append(word, r)
			continue
		case unicode.Is(unicode.Mn, r) && len(word) > 0:
			// A combining accent written apart from its letter.
			word = append(word, r)
			continue
		}
		flush()
		switch {
		case unicode.In(r, unicode.Han, unicode.Hangul) && unicode.IsLetter(r):
			n++
		case unicode.In(r, unicode.Hiragana, unicode.Katakana) || r == 'ー':
			if !strings.ContainsRune(smallKana, r) && unicode.IsLetter(r) {
				n++
			}
		case unicode.IsDigit(r):
			n++
		case unicode.In(r, unicode.Thai, unicode.Lao, unicode.Khmer, unicode.Myanmar):
			if southeastConsonant(r) {
				southeast++
			}
		case unicode.In(r, unicode.Devanagari, unicode.Bengali, unicode.Gurmukhi, unicode.Gujarati, unicode.Oriya, unicode.Tamil, unicode.Telugu, unicode.Kannada, unicode.Malayalam, unicode.Sinhala):
			if unicode.IsLetter(r) {
				indic++
			} else if isVirama(r) {
				indic--
			}
		case unicode.In(r, unicode.Hebrew, unicode.Arabic, unicode.Syriac, unicode.Thaana):
			if unicode.IsLetter(r) {
				abjad++
			}
		case unicode.IsLetter(r):
			// A script of its own: a letter run is a word, a syllable at least.
			word = append(word, r)
		}
	}
	flush()
	return n + southeast/1.6 + max(0, indic) + abjad/2
}

// isVowel reports whether the lower-case letter r is a vowel of an alphabet syllables counts vowel groups in, with or without its accents: Latin, as Vietnamese writes it too, Cyrillic and Greek.
func isVowel(r rune) bool {
	// Vietnamese's vowels with their tone marks.
	return r >= 0x1EA0 && r <= 0x1EF9 || strings.ContainsRune(syllableVowels, r)
}

// syllableVowels are the vowels isVowel knows apart from Vietnamese's.
const syllableVowels = "aeiouyæøœɛɔəɪʊʌɑɒ" + "àáâãäåèéêëìíîïòóôõöøùúûüýÿ" + "āăąēĕėęěĩīĭįıōŏőũūŭůűųŷơưǎǐǒǔǖǘǚǜ" + "аеёиоуыэюяіїє" + "αεηιουωάέήίόύώϊϋΐΰ"

// smallKana are the kana said with the one before them, and the mark that lengthens one, which add no syllable.
const smallKana = "ぁぃぅぇぉっゃゅょゎゕゖァィゥェォッャュョヮヵヶㇰㇱㇲㇳㇴㇵㇶㇷㇸㇹㇺㇻㇼㇽㇾㇿー"

// southeastConsonant reports whether r is a consonant letter of Thai, Lao, Khmer or Burmese, which write a syllable around one or two of them.
func southeastConsonant(r rune) bool {
	return r >= 0x0E01 && r <= 0x0E2E || r >= 0x0E81 && r <= 0x0EAE || r >= 0x1780 && r <= 0x17A2 || r >= 0x1000 && r <= 0x102A
}

// isVirama reports whether r is the mark of an Indic script that takes a consonant's vowel away, joining it to the next.
func isVirama(r rune) bool {
	switch r {
	case 0x094D, 0x09CD, 0x0A4D, 0x0ACD, 0x0B4D, 0x0BCD, 0x0C4D, 0x0CCD, 0x0D4D, 0x0DCA:
		return true
	}
	return false
}
