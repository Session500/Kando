package films

import (
	"path/filepath"
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
