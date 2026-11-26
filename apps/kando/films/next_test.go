package films

import (
	"path/filepath"
	"strings"
	"testing"

	"kando/packages/core"
)

// local is a done local film at path, keyed by its file's name.
func local(path string) Film {
	return Film{Key: filepath.Base(path), VideoPath: path, Source: core.FileURL(path), Stage: core.StageComplete}
}

// nextKeys is the keys Next steps through from the film keyed from, among all.
func nextKeys(all []Film, from string) []string {
	item, _ := Find(all, from)
	var out []string
	for {
		next, ok := Next(all, item, "zh-Hans")
		if !ok {
			return out
		}
		out = append(out, next.Key)
		item = next
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A course's lessons go on by number, Chinese numerals by value, and into the next section's folder; a folder's other files, and its clips that number nothing, are not played after them.
func TestNextGoesOnThroughACourse(t *testing.T) {
	root := "/Courses/日语"
	all := []Film{
		local(root + "/第1节/日语课 第二讲.mp4"),
		local(root + "/第1节/日语课 第十一讲.mp4"),
		local(root + "/第1节/日语课 第一讲.mp4"),
		local(root + "/第1节/notes clip.mp4"),
		local(root + "/第10节/日语课 第十三讲.mp4"),
		local(root + "/第2节/日语课 第十二讲.mp4"),
	}
	got := nextKeys(all, "日语课 第一讲.mp4")
	want := []string{"日语课 第二讲.mp4", "日语课 第十一讲.mp4", "日语课 第十二讲.mp4", "日语课 第十三讲.mp4"}
	if !equal(got, want) {
		t.Fatalf("next %q, want %q", got, want)
	}
	if _, ok := Next(all, all[3], "zh-Hans"); ok {
		t.Fatal("a clip that numbers nothing has a next")
	}
	prev, ok := Previous(all, all[1], "zh-Hans")
	if !ok || prev.Key != "日语课 第二讲.mp4" {
		t.Fatalf("previous %+v %v", prev, ok)
	}
}

// A disc's tracks go on by their leading numbers; clips of other shows beside them, as in a Downloads folder, are not played after them, nor is a downloaded video whose title ends in a number.
func TestNextKeepsToOneShow(t *testing.T) {
	all := []Film{
		local("/Downloads/02 Dialogue.mp3"),
		local("/Downloads/01 Opening.mp3"),
		local("/Downloads/10 Review.mp3"),
		local("/Downloads/Cooking Show EP02.mp4"),
		local("/Downloads/Cooking Show EP01.mp4"),
		local("/Downloads/Trip to Kyoto.mp4"),
		{Key: "dl", VideoPath: "/Library/Talk 3 [abc]/video.mp4", Source: "https://www.youtube.com/watch?v=abc", Stage: core.StageComplete},
		{Key: "dl2", VideoPath: "/Library/Talk 4 [abd]/video.mp4", Source: "https://www.youtube.com/watch?v=abd", Stage: core.StageComplete},
	}
	if got, want := nextKeys(all, "01 Opening.mp3"), []string{"02 Dialogue.mp3", "10 Review.mp3"}; !equal(got, want) {
		t.Fatalf("tracks %q, want %q", got, want)
	}
	if got, want := nextKeys(all, "Cooking Show EP01.mp4"), []string{"Cooking Show EP02.mp4"}; !equal(got, want) {
		t.Fatalf("show %q, want %q", got, want)
	}
	for _, key := range []string{"Trip to Kyoto.mp4", "dl"} {
		item, _ := Find(all, key)
		if next, ok := Next(all, item, "en"); ok {
			t.Fatalf("%s has next %s", key, next.Key)
		}
	}
}

// A list's films go on in the list's order, wherever their files are, those with no place yet after the rest; a film that failed is passed over, and one still waiting is next all the same.
func TestNextFollowsTheList(t *testing.T) {
	video := func(key string, index int, stage core.Stage) Film {
		return Film{Key: key, List: "youtube.com/playlist?list=PL1", ListIndex: index, Source: "https://www.youtube.com/watch?v=" + key, Stage: stage,
			VideoPath: "/Library/" + key + " [" + key + "]/video.mp4"}
	}
	all := []Film{
		video("e5", 5, core.StageComplete),
		video("e1", 1, core.StageComplete),
		video("e2", 2, core.StageFailed),
		video("e3", 3, core.StageQueued),
		video("extra", 0, core.StageComplete),
		{Key: "other", List: "youtube.com/playlist?list=PL2", ListIndex: 2, Stage: core.StageComplete},
	}
	if got, want := nextKeys(all, "e1"), []string{"e3", "e5", "extra"}; !equal(got, want) {
		t.Fatalf("next %q, want %q", got, want)
	}
	e3, _ := Find(all, "e3")
	if prev, ok := Previous(all, e3, "en"); !ok || prev.Key != "e1" {
		t.Fatalf("previous %+v %v", prev, ok)
	}
}

// Nexts gives every film's next as Next does, a library at once.
func TestNextsIsNextForEveryFilm(t *testing.T) {
	all := []Film{
		local("/Courses/a/Lesson 02.mp4"), local("/Courses/a/Lesson 01.mp4"), local("/Courses/b/Lesson 03.mp4"),
		local("/Downloads/Trip.mp4"),
		{Key: "e2", List: "L", ListIndex: 2, Stage: core.StageComplete}, {Key: "e1", List: "L", ListIndex: 1, Stage: core.StageComplete},
		{Key: "e3", List: "L", ListIndex: 3, Stage: core.StageFailed}, {Key: "e4", List: "L", ListIndex: 4, Stage: core.StageComplete},
	}
	got := Nexts(all, "en")
	for _, item := range all {
		want, ok := Next(all, item, "en")
		if g, has := got[item.Key]; has != ok || g.Key != want.Key {
			t.Errorf("%s: Nexts %q %v, Next %q %v", item.Key, g.Key, has, want.Key, ok)
		}
	}
}

// A show's folders go in one order whatever order its films come in: the numbered ones first by number, then the others by name, so its films do not interleave folder by folder.
func TestNextOrdersAShowsFoldersTheSameWhateverTheirOrder(t *testing.T) {
	root := "/Courses/课"
	var all []Film
	for _, folder := range []string{"Basics 2", "10 Advanced", "Advanced"} {
		for _, lesson := range []string{"Lesson 01", "Lesson 02"} {
			all = append(all, local(root+"/"+folder+"/"+lesson+".mp4"))
		}
	}
	want := []string{"Basics 2/Lesson 01", "Basics 2/Lesson 02", "10 Advanced/Lesson 01", "10 Advanced/Lesson 02", "Advanced/Lesson 01", "Advanced/Lesson 02"}
	orders := [][]int{{0, 1, 2, 3, 4, 5}, {5, 4, 3, 2, 1, 0}, {2, 0, 4, 1, 5, 3}, {4, 2, 0, 5, 3, 1}}
	for _, order := range orders {
		films := make([]Film, len(all))
		for i, j := range order {
			films[i] = all[j]
			films[i].Key = filepath.Base(filepath.Dir(all[j].VideoPath)) + "/" + strings.TrimSuffix(filepath.Base(all[j].VideoPath), ".mp4")
		}
		got := append([]string{"Basics 2/Lesson 01"}, nextKeys(films, "Basics 2/Lesson 01")...)
		if !equal(got, want) {
			t.Fatalf("films in order %v play %q, want %q", order, got, want)
		}
	}
}

// A show's seasons in folders named 第一季 … 第十一季 go by number, in every language of the interface, not by how each language sorts Chinese numerals.
func TestNextGoesOnThroughSeasonsNamedInChinese(t *testing.T) {
	root := "/剧集/庆余年"
	seasons := []string{"第十一季", "第二季", "第十季", "第一季", "第四季"}
	var all []Film
	for _, season := range seasons {
		f := local(root + "/" + season + "/庆余年 第01集.mp4")
		f.Key = season
		all = append(all, f)
	}
	want := []string{"第二季", "第四季", "第十季", "第十一季"}
	for _, tag := range []string{"zh-Hans", "zh-Hant", "en", "ja", "ko"} {
		item, _ := Find(all, "第一季")
		var got []string
		for {
			next, ok := Next(all, item, tag)
			if !ok {
				break
			}
			got = append(got, next.Key)
			item = next
		}
		if !equal(got, want) {
			t.Fatalf("in %s the seasons play %q, want %q", tag, got, want)
		}
	}
}

// A fansub's releases, numbered by a tag of their own or with underscores for spaces, go on by number as other episodes do.
func TestNextGoesOnThroughAFansubsReleases(t *testing.T) {
	all := []Film{
		local("/Anime/山海旅人/[字幕组][山海旅人][02][1080P].mkv"),
		local("/Anime/山海旅人/[字幕组][山海旅人][10][1080P].mkv"),
		local("/Anime/山海旅人/[字幕组][山海旅人][01][1080P].mkv"),
		local("/Anime/River/River_Walk_E02_1080p.mkv"),
		local("/Anime/River/River_Walk_E01_1080p.mkv"),
	}
	if got, want := nextKeys(all, "[字幕组][山海旅人][01][1080P].mkv"), []string{"[字幕组][山海旅人][02][1080P].mkv", "[字幕组][山海旅人][10][1080P].mkv"}; !equal(got, want) {
		t.Fatalf("next %q, want %q", got, want)
	}
	if got, want := nextKeys(all, "River_Walk_E01_1080p.mkv"), []string{"River_Walk_E02_1080p.mkv"}; !equal(got, want) {
		t.Fatalf("next %q, want %q", got, want)
	}
}
