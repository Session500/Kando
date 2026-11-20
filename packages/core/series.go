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
	name = strings.TrimSpace(episodeTags.ReplaceAllString(name, " "))
	for _, mark := range episodeMarks {
		m := mark.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		show = strings.Trim(episodeSpace.ReplaceAllString(m[1], " "), " -–—:：,，")
		if utf8.RuneCountInString(show) < 2 {
			return "", "", false
		}
		return show, strings.ToLower(show), true
	}
	return "", "", false
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
	name = strings.TrimSpace(episodeTags.ReplaceAllString(name, " "))
	if m := episodeSeason.FindStringSubmatch(name); m != nil {
		season, _ = strconv.Atoi(m[1])
		episode, _ = strconv.Atoi(m[2])
		return season, episode, true
	}
	for _, re := range episodeNumbers {
		if m := re.FindStringSubmatch(name); m != nil {
			if n, ok := episodeValue(m[1]); ok {
				return 0, n, true
			}
		}
	}
	return 0, 0, false
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
