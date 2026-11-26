package core

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Episodes: films whose names say they are episodes of one show, as "Planet Earth S01E02.mkv", "地理课 第3集" or "Lecture 04", are likely a series. EpisodeOf reads the show's name from a film's name, so the app can ask the user whether films that share it are one series; it only reads names, so it never puts films in a series without asking.

var (
	// episodeTags are the bracketed tags release names carry, as [1080p] or (2024), which are not the show's name.
	episodeTags = regexp.MustCompile(`[\[【(（][^\]】)）]*[\]】)）]`)
	// episodeMarks are the ways a name marks an episode, each with the show's name before it: S01E02, EP02 or Episode 2, 第2集, and a number at the end after a separator.
	episodeMarks = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^(.*?)[\s._-]*\bS\d{1,2}[\s._-]*E\d{1,3}\b`),
		regexp.MustCompile(`(?i)^(.*?)[\s._-]*\b(?:EP|E|Episode|Ep\.)\s*\d{1,3}\b`),
		regexp.MustCompile(`^(.*?)\s*第\s*[0-9０-９一二三四五六七八九十百]+\s*[集话話回期讲講課课]`),
		regexp.MustCompile(`^(.*?)[\s._-]+(?:#|No\.?\s*|Part\s*|Vol\.?\s*)?\d{1,3}$`),
	}
	// episodeSpace are the separators release names write for spaces.
	episodeSpace = regexp.MustCompile(`[\s._]+`)
)

// EpisodeOf is the name of the show the film named name, without its file extension, is an episode of, as name marks it, and the key films of the same show share whatever their case and separators; ok is false for a name that marks no episode, or leaves too little of a name before its mark.
func EpisodeOf(name string) (show, key string, ok bool) {
	stripped := withoutTags(name)
	show, ok = showIn(stripped)
	if !ok {
		// A name read as numbering an episode is read as it was; only one read as numbering none is read as a fansub's might be.
		if _, _, numbered := numberIn(stripped); !numbered {
			show, _, _, _ = fansubEpisode(name)
			ok = show != ""
		}
	}
	if !ok {
		return "", "", false
	}
	return show, strings.ToLower(show), true
}

// withoutTags is name without the bracketed tags release names carry (episodeTags).
func withoutTags(name string) string {
	return strings.TrimSpace(episodeTags.ReplaceAllString(name, " "))
}

// showIn is the show's name before the mark of an episode in name, a name without its tags; false when none marks one, or too little of a name is left before it.
func showIn(name string) (string, bool) {
	for _, mark := range episodeMarks {
		m := mark.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		return showName(m[1])
	}
	return "", false
}

// showName is a show's name as a name writes it, its separators made spaces; false for one too short to be a name.
func showName(s string) (string, bool) {
	show := strings.Trim(episodeSpace.ReplaceAllString(s, " "), " -–—:：,，")
	if utf8.RuneCountInString(show) < 2 {
		return "", false
	}
	return show, true
}

var (
	// episodeNumbers are the ways a name numbers an episode, as episodeMarks find them, each with the number: S01E02 with its season, EP02 or Episode 2, 第2集 or 第十一讲, a number at the end after a separator, as "Lecture 04" or "Track 02", and one at the start, as "01 Opening" a disc's tracks are named.
	episodeSeason  = regexp.MustCompile(`(?i)\bS(\d{1,2})[\s._-]*E(\d{1,3})\b`)
	episodeNumbers = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:EP|E|Episode|Ep\.)\s*(\d{1,3})\b`),
		regexp.MustCompile(`第\s*([0-9０-９一二三四五六七八九十百〇零两]+)\s*[集话話回期讲講課课章节節]`),
		regexp.MustCompile(`[\s._-](?:#|No\.?\s*|Part\s*|Vol\.?\s*)?(\d{1,3})$`),
		regexp.MustCompile(`^(\d{1,3})(?:[\s._-]|$)`),
	}
)

// EpisodeNumber is the season and the episode a film's name, without its file extension, numbers it as, as EpisodeOf reads it: S01E02 is season 1 and episode 2, a name without a season has season 0, and 第十一讲 is episode 11. ok is false for a name that numbers nothing. It orders a show's films by number, where their names would order 第十一讲 before 第二讲 and "Episode 10" before "Episode 9".
func EpisodeNumber(name string) (season, episode int, ok bool) {
	if season, episode, ok = numberIn(withoutTags(name)); ok {
		return season, episode, true
	}
	_, season, episode, ok = fansubEpisode(name)
	return season, episode, ok
}

// numberIn is the season and the episode name, a name without its tags, numbers, as EpisodeNumber reads them.
func numberIn(name string) (season, episode int, ok bool) {
	season, episode, _, _, ok = markIn(name)
	return season, episode, ok
}

// markIn is numberIn with how name writes the episode's number: its digits as written, and whether a mark says it is an episode's, as S01E02, EP02 and 第2集 do, rather than a number alone at the name's end or start.
func markIn(name string) (season, episode int, digits string, explicit, ok bool) {
	if m := episodeSeason.FindStringSubmatch(name); m != nil {
		season, _ = strconv.Atoi(m[1])
		episode, _ = strconv.Atoi(m[2])
		return season, episode, m[2], true, true
	}
	for i, re := range episodeNumbers {
		if m := re.FindStringSubmatch(name); m != nil {
			if n, ok := episodeValue(m[1]); ok {
				return 0, n, m[1], i < episodeMarked, true
			}
		}
	}
	return 0, 0, "", false, false
}

// episodeMarked is how many of episodeNumbers, the first, read a number a mark says is an episode's.
const episodeMarked = 2

// EpisodeMark is how the film named name, without its file extension, writes the number EpisodeNumber reads: its digits as written, as "01" or "十一", and whether a mark says it is an episode's (S01E02, EP02, Episode 2, 第2集) rather than a number alone, as "Track 02", "01 Opening" and a fansub's [01] are. ok is false for a name that numbers nothing.
func EpisodeMark(name string) (digits string, explicit, ok bool) {
	if _, _, digits, explicit, ok = markIn(withoutTags(name)); ok {
		return digits, explicit, true
	}
	_, _, _, digits, explicit, ok = fansubMark(name)
	return digits, explicit, ok
}

var (
	// fansubNumber is an episode's number as a fansub's release names it, a tag of its own: [01], 【12】 or [03v2]. A number in parentheses is not one, as a copy's "Holiday (2)" is not an episode.
	fansubNumber = regexp.MustCompile(`[\[【]\s*(\d{1,3})(?:v\d)?\s*[\]】]`)
	// fansubTag is a bracketed tag a fansub's release names hold, as its group's and the show's: [Group][Show][01][1080P].
	fansubTag = regexp.MustCompile(`[\[【]([^\]】]*)[\]】]`)
	// fansubEnd is the separators a name may end in once its tags are taken out, as "Show_-_01_[1080p]" does.
	fansubEnd = regexp.MustCompile(`[\s._-]+$`)
)

// fansubEpisode reads the show and the episode of a name the other ways do not, as fansubs' and others' releases name them: with underscores for spaces (Frieren_E01_1080p), separators left after a tag (Show_-_01_[1080p]), or the episode's number a bracketed tag of its own after the group's and the show's ([Group][Show][01][1080P]). It reads only names that number nothing as EpisodeNumber reads them otherwise, so a name read before is read as it was. show is "" for a name that numbers an episode without a show's name.
func fansubEpisode(name string) (show string, season, episode int, ok bool) {
	show, season, episode, _, _, ok = fansubMark(name)
	return show, season, episode, ok
}

// fansubMark is fansubEpisode with how the name writes the episode's number, as markIn says it.
func fansubMark(name string) (show string, season, episode int, digits string, explicit, ok bool) {
	stripped := withoutTags(name)
	spaced := strings.TrimSpace(fansubEnd.ReplaceAllString(strings.ReplaceAll(stripped, "_", " "), ""))
	if spaced != stripped {
		if season, episode, digits, explicit, ok = markIn(spaced); ok {
			show, _ = showIn(spaced)
			return show, season, episode, digits, explicit, true
		}
	}
	at := fansubNumber.FindStringSubmatchIndex(name)
	if at == nil {
		return "", 0, 0, "", false, false
	}
	digits = name[at[2]:at[3]]
	episode, _ = strconv.Atoi(digits)
	// The show's name is the text before the number, outside its tags; else the last tag before it, the first being the group's. Text after it is the episode's title, as "[01] Intro" names one.
	before := name[:at[0]]
	if show, ok := showName(strings.ReplaceAll(withoutTags(before), "_", " ")); ok {
		return show, 0, episode, digits, false, true
	}
	tags := fansubTag.FindAllStringSubmatch(before, -1)
	for i := len(tags) - 1; i >= 1; i-- {
		if show, ok := showName(tags[i][1]); ok {
			return show, 0, episode, digits, false, true
		}
	}
	return "", 0, episode, digits, false, true
}

// seasonMark is how a folder's name numbers a season or a part of a show, as 第二季, 第3部 or シーズン2, which EpisodeNumber does not read, since in a film's name 季 comes before the episode's own number.
var seasonMark = regexp.MustCompile(`(?:第\s*([0-9０-９一二三四五六七八九十百〇零两]+)\s*[季部]|シーズン\s*([0-9０-９]+))`)

// SeasonNumber is the season or part a folder's name numbers, as 第二季 is 2 and シーズン3 is 3; ok is false for a name that numbers none this way.
func SeasonNumber(name string) (int, bool) {
	m := seasonMark.FindStringSubmatch(name)
	if m == nil {
		return 0, false
	}
	digits := m[1]
	if digits == "" {
		digits = m[2]
	}
	return episodeValue(digits)
}

// episodeValue is the number s writes: digits, full-width digits, or Chinese numerals up to the hundreds, as 十一, 二十 and 一百零五.
func episodeValue(s string) (int, bool) {
	digits := map[rune]int{'〇': 0, '零': 0, '一': 1, '二': 2, '两': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	total, digit, chinese := 0, 0, false
	plain := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			plain = plain*10 + int(r-'0')
		case r >= '０' && r <= '９':
			plain = plain*10 + int(r-'０')
		case r == '十', r == '百':
			chinese = true
			if digit == 0 {
				digit = 1
			}
			if r == '十' {
				total += digit * 10
			} else {
				total += digit * 100
			}
			digit = 0
		default:
			d, ok := digits[r]
			if !ok {
				return 0, false
			}
			chinese = true
			digit = d
		}
	}
	if chinese {
		return total + digit, true
	}
	return plain, s != ""
}

// sectionMarks are how a folder's name says it holds a season or a part of a show or a course, at its start, a title after it or none: Season 2, S01, Part 3, Disc 1, CD2, Vol. 4, Section 1 - Introduction, Chapter 5, Week 6, Module 7, Unit 8, 第二季, 第3节 or シーズン2. A name it misses only leaves its films a card of their own, so it lists the ways rather than guess.
var sectionMarks = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(?:Season|Series|Part|Disc|CD|Vol\.?|Volume|Section|Chapter|Week|Module|Unit)[\s._-]*\d{1,3}(?:$|[\s._:：-])`),
	regexp.MustCompile(`(?i)^S\d{1,2}(?:$|[\s._:：-])`),
	regexp.MustCompile(`^第\s*[0-9０-９一二三四五六七八九十百〇零两]+\s*[季部节節章篇]`),
	regexp.MustCompile(`^シーズン\s*[0-9０-９]+`),
}

// SectionFolder reports whether a folder's name says it holds a season or a part of what its parent folder holds (sectionMarks), its tags left out.
func SectionFolder(name string) bool {
	name = withoutTags(name)
	for _, mark := range sectionMarks {
		if mark.MatchString(name) {
			return true
		}
	}
	return false
}

// bareEpisode is a name, its tags left out, that is an episode's number and nothing else: 01, EP05, E3, #12, No. 4, S01E02 or 第3话.
var bareEpisode = regexp.MustCompile(`(?i)^(?:S\d{1,2}[\s._-]*E\d{1,3}|(?:EP|E|Episode|Ep\.|#|No\.?)?[\s._-]*\d{1,3}|第\s*[0-9０-９一二三四五六七八九十百〇零两]+\s*[集话話回期讲講課课章节節])$`)

// BareEpisode reports whether the film named name, without its file extension, is named by its episode's number alone, its tags left out, as 01, EP05 or 第3话 are, so its name says nothing of what it is an episode of.
func BareEpisode(name string) bool {
	if _, _, ok := EpisodeNumber(name); !ok {
		return false
	}
	rest := withoutTags(name)
	return rest == "" || bareEpisode.MatchString(rest)
}

// WithoutTags is name without the bracketed tags release names carry, as [1080p], 【字幕组】 or (2024).
func WithoutTags(name string) string { return strings.Join(strings.Fields(withoutTags(name)), " ") }
