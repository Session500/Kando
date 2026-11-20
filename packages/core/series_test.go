package core

import "testing"

func TestEpisodeOf(t *testing.T) {
	for _, tc := range []struct{ name, show string }{
		{"Planet.Earth.S01E02.1080p", "Planet Earth"},
		{"planet earth s02e10", "planet earth"},
		{"[Group] Planet Earth - EP03 [1080p]", "Planet Earth"},
		{"Lecture Series Episode 4", "Lecture Series"},
		{"地理课 第3集", "地理课"},
		{"地理课第十二讲", "地理课"},
		{"Physics 101 - 04", "Physics 101"},
		{"Cooking with Ana #12", "Cooking with Ana"},
		{"Tokyo Story", ""},
		{"E3 2024", ""},
		{"第3集", ""},
	} {
		show, key, ok := EpisodeOf(tc.name)
		if show != tc.show || ok != (tc.show != "") {
			t.Errorf("EpisodeOf(%q) = %q, %v, want %q", tc.name, show, ok, tc.show)
		}
		if ok && key == "" {
			t.Errorf("EpisodeOf(%q) has no key", tc.name)
		}
	}
	_, a, _ := EpisodeOf("Planet.Earth.S01E02")
	_, b, _ := EpisodeOf("planet earth - EP 7")
	if a != b {
		t.Errorf("the same show has keys %q and %q", a, b)
	}
}

// A film's name numbers it in every way a show's names do, and Chinese numerals and full-width digits read as their values.
func TestEpisodeNumber(t *testing.T) {
	for name, want := range map[string][3]int{
		"Planet Earth S01E02":        {1, 2, 1},
		"Planet.Earth.s02e10.1080p":  {2, 10, 1},
		"Show EP09":                  {0, 9, 1},
		"Episode 10":                 {0, 10, 1},
		"地理课 第3集":                    {0, 3, 1},
		"日语课 第十一讲":                   {0, 11, 1},
		"日语课 第二十讲":                   {0, 20, 1},
		"有声书 第一百零五章":                 {0, 105, 1},
		"第１２回":                       {0, 12, 1},
		"Lecture 04":                 {0, 4, 1},
		"N3_Listening_07":            {0, 7, 1},
		"Track 02":                   {0, 2, 1},
		"01 Opening":                 {0, 1, 1},
		"12. Dialogue [2024]":        {0, 12, 1},
		"A Talk About Rivers":        {0, 0, 0},
		"2024 Highlights of the Sea": {0, 0, 0},
	} {
		season, episode, ok := EpisodeNumber(name)
		got := [3]int{season, episode, 0}
		if ok {
			got[2] = 1
		}
		if got != want {
			t.Errorf("EpisodeNumber(%q) = %v, want %v", name, got, want)
		}
	}
}
