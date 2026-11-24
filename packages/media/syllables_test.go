package media

import (
	"math"
	"testing"
)

func TestSyllablesCountsWhatSayingTakesInAnyScript(t *testing.T) {
	for _, tc := range []struct {
		text string
		want float64
	}{
		{"今日はいい天気ですね", 10},
		// Small kana and the long mark are said with the kana before them.
		{"ちょっと待ってください", 8},
		{"コーヒーを飲みませんか", 9},
		{"오늘 날씨가 좋네요", 8},
		{"今天天气很好", 6},
		// A word written in an alphabet is its vowel groups, accents or tone marks on them or not.
		{"I would like a cup of tea", 8},
		{"¿Dónde está la estación?", 8},
		{"Hôm nay trời đẹp quá", 5},
		{"Привет, как дела?", 5},
		{"आज मौसम अच्छा है", 8},
		{"الطقس جميل اليوم", 7},
		{"วันนี้อากาศดีมาก", 5.625},
		{"1897年に生まれた", 10},
		{"- ...", 0},
	} {
		if got := syllables(tc.text); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("syllables(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}
