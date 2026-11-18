package core

import "testing"

// TestSameLanguage checks which films need no translation: one heard in the language its subtitles are for, Chinese only in the script the target names; a film heard in another language, or in one Kando cannot tell, is translated.
func TestSameLanguage(t *testing.T) {
	simplified := []Segment{{Text: "这个问题我们来说一说，学会了就记住"}}
	traditional := []Segment{{Text: "這個問題我們來說一說，學會了就記住"}}
	for _, tc := range []struct {
		name           string
		spoken, target string
		lines          []Segment
		want           bool
	}{
		{"Mandarin for 简体中文", "zh", "简体中文", simplified, true},
		{"Mandarin written Traditional for 简体中文", "zh", "简体中文", traditional, false},
		{"Mandarin for 繁體中文", "zh", "繁體中文", simplified, false},
		{"Mandarin written Traditional for 繁體中文", "zh", "繁體中文", traditional, true},
		{"English for English", "en", "English", nil, true},
		{"English for 简体中文", "en", "简体中文", nil, false},
		{"Cantonese for 简体中文", "yue", "简体中文", simplified, false},
		{"heard nothing", "", "简体中文", simplified, false},
		{"a target Kando does not know", "zh", "文言文", simplified, false},
	} {
		if got := SameLanguage(tc.spoken, tc.target, tc.lines); got != tc.want {
			t.Errorf("%s: SameLanguage = %v, want %v", tc.name, got, tc.want)
		}
	}
}
