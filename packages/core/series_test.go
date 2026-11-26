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

func TestSeasonNumber(t *testing.T) {
	for name, want := range map[string]int{"第二季": 2, "第十一季": 11, "第3部": 3, "シーズン2": 2, "庆余年 第二季": 2} {
		if got, ok := SeasonNumber(name); !ok || got != want {
			t.Errorf("SeasonNumber(%q) = %d, %v, want %d", name, got, ok, want)
		}
	}
	for _, name := range []string{"Season 2", "第2集", "季", "Extras"} {
		if got, ok := SeasonNumber(name); ok {
			t.Errorf("SeasonNumber(%q) = %d, want none", name, got)
		}
	}
}

// Names the other ways read nothing in, as fansubs' releases name them, read too: with underscores for spaces, separators left after a tag, or the episode's number a bracketed tag of its own.
func TestEpisodeOfFansubNames(t *testing.T) {
	for _, tc := range []struct {
		name, show string
		episode    int
	}{
		{"[字幕组][山海旅人][01][1080P]", "山海旅人", 1},
		{"【字幕组】【山海旅人】【12】【简体】", "山海旅人", 12},
		{"[Group] River Walk [03v2][1080p]", "River Walk", 3},
		{"River_Walk_-_04_[1080p]", "River Walk", 4},
		{"River_Walk_E05_1080p", "River Walk", 5},
		{"[07] Opening Night", "", 7},
		{"[Group][08]", "", 8},
	} {
		_, episode, ok := EpisodeNumber(tc.name)
		if !ok || episode != tc.episode {
			t.Errorf("EpisodeNumber(%q) = %d, %v, want %d", tc.name, episode, ok, tc.episode)
		}
		show, _, ok := EpisodeOf(tc.name)
		if show != tc.show || ok != (tc.show != "") {
			t.Errorf("EpisodeOf(%q) = %q, %v, want %q", tc.name, show, ok, tc.show)
		}
	}
	// A copy's number in parentheses, and a tag of a year or a resolution, number nothing.
	for _, name := range []string{"Holiday (2)", "Holiday [2024]", "Holiday [1080p]", "IMG_1234", "Holiday_"} {
		if _, episode, ok := EpisodeNumber(name); ok {
			t.Errorf("EpisodeNumber(%q) = %d, want none", name, episode)
		}
	}
}

func TestSectionFolder(t *testing.T) {
	for _, name := range []string{"Season 2", "season 01", "S01", "Part 3", "Disc 1", "CD2", "Vol. 4", "Section 1 - Introduction", "Chapter 5", "Week 6", "Module 7", "Unit 8", "第二季", "第3节", "シーズン2", "[BD] Season 1"} {
		if !SectionFolder(name) {
			t.Errorf("SectionFolder(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"Extras", "30 Days of Japanese", "Hits Vol. 2 Live", "Seasons", "Specials", "Downloads", "Show Season 2", "Sunday"} {
		if SectionFolder(name) {
			t.Errorf("SectionFolder(%q) = true, want false", name)
		}
	}
}

func TestBareEpisode(t *testing.T) {
	for _, name := range []string{"01", "EP05", "E3", "S01E02", "第3话", "[Group][08]", "01 [1080p]"} {
		if !BareEpisode(name) {
			t.Errorf("BareEpisode(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"01 Opening", "Lecture 04", "River Walk EP05", "Tokyo Story", "[字幕组][山海旅人][01] 海边"} {
		if BareEpisode(name) {
			t.Errorf("BareEpisode(%q) = true, want false", name)
		}
	}
}

func TestEpisodeMark(t *testing.T) {
	for name, want := range map[string]struct {
		digits   string
		explicit bool
	}{
		"Show S01E02": {"02", true}, "Show EP09": {"09", true}, "地理课 第十一讲": {"十一", true},
		"Track 02": {"02", false}, "01 Opening": {"01", false}, "[字幕组][山海旅人][07][1080P]": {"07", false}, "River_Walk_E05_1080p": {"05", true},
	} {
		digits, explicit, ok := EpisodeMark(name)
		if !ok || digits != want.digits || explicit != want.explicit {
			t.Errorf("EpisodeMark(%q) = %q, %v, %v, want %q, %v", name, digits, explicit, ok, want.digits, want.explicit)
		}
	}
	if _, _, ok := EpisodeMark("Tokyo Story"); ok {
		t.Error("a name that numbers nothing has a mark")
	}
}
