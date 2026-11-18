package core

import "strings"

// targetTags are the languages the settings translate into (settings › 翻译 › 默认目标语言), by the name a job keeps as its Language, as BCP 47 tags: Chinese names its script, as a film heard in Chinese is already in one of the two only when its transcript is written in that script.
var targetTags = map[string]string{
	"简体中文": "zh-Hans", "繁体中文": "zh-Hant", "繁體中文": "zh-Hant", "English": "en", "日本語": "ja", "한국어": "ko", "Español": "es", "Français": "fr", "Deutsch": "de",
	"Português": "pt", "Italiano": "it", "Русский": "ru", "Nederlands": "nl", "Polski": "pl", "Українська": "uk", "Türkçe": "tr", "العربية": "ar", "हिन्दी": "hi",
	"Bahasa Indonesia": "id", "Tiếng Việt": "vi", "ไทย": "th",
}

// TargetTag is the BCP 47 tag of the target language a job's Language names, such as zh-Hans for 简体中文, "" for one Kando does not know, as a language typed into a custom prompt.
func TargetTag(target string) string { return targetTags[strings.TrimSpace(target)] }

// BaseLanguage is the language of tag without its script or region, as whisper names what it heard: zh for zh-Hans.
func BaseLanguage(tag string) string {
	base, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(tag)), "-")
	return base
}

// SameLanguage reports whether a film whose speech was heard in spoken (whisper's code, as Job.SpokenLanguage) and transcribed as lines is already in target (a Job.Language): its subtitles are its own lines and need no translation. Chinese is the same only in the script the target names, which the lines are told to be written in by the characters the two scripts write differently (ChineseScript); lines that write neither are taken as Simplified, as whisper writes Mandarin. An unknown spoken language or target is never the same, so such a film is translated as before.
func SameLanguage(spoken, target string, lines []Segment) bool {
	tag := TargetTag(target)
	spoken = BaseLanguage(spoken)
	if tag == "" || spoken == "" || BaseLanguage(tag) != spoken {
		return false
	}
	if spoken != "zh" {
		return true
	}
	return ChineseScript(lines) == tag
}

// scriptPairs are common characters Simplified and Traditional Chinese write differently, Simplified first: enough of them turn up in any minute of speech to tell which script a transcript is in.
var scriptPairs = [][2]rune{
	{'这', '這'}, {'个', '個'}, {'们', '們'}, {'说', '說'}, {'时', '時'}, {'会', '會'}, {'来', '來'}, {'对', '對'},
	{'为', '為'}, {'过', '過'}, {'还', '還'}, {'没', '沒'}, {'么', '麼'}, {'国', '國'}, {'学', '學'}, {'后', '後'},
	{'里', '裡'}, {'问', '問'}, {'题', '題'}, {'开', '開'}, {'关', '關'}, {'点', '點'}, {'样', '樣'}, {'经', '經'},
	{'现', '現'}, {'间', '間'}, {'让', '讓'}, {'听', '聽'}, {'见', '見'}, {'长', '長'}, {'动', '動'}, {'号', '號'},
	{'从', '從'}, {'边', '邊'}, {'东', '東'}, {'车', '車'}, {'话', '話'}, {'应', '應'}, {'该', '該'}, {'记', '記'},
	{'谁', '誰'}, {'认', '認'}, {'识', '識'}, {'办', '辦'}, {'发', '發'}, {'头', '頭'}, {'两', '兩'}, {'几', '幾'},
}

var simplified, traditional = func() (map[rune]bool, map[rune]bool) {
	s, t := map[rune]bool{}, map[rune]bool{}
	for _, p := range scriptPairs {
		s[p[0]], t[p[1]] = true, true
	}
	return s, t
}()

// ChineseScript is the script Chinese lines are written in, zh-Hant when they write more of the characters only Traditional Chinese writes (scriptPairs) than of those only Simplified does, else zh-Hans.
func ChineseScript(lines []Segment) string {
	var s, t int
	for _, line := range lines {
		for _, r := range line.Text {
			switch {
			case simplified[r]:
				s++
			case traditional[r]:
				t++
			}
		}
	}
	if t > s {
		return "zh-Hant"
	}
	return "zh-Hans"
}
