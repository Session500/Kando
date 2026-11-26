package shelf

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"kando/packages/core"
)

// The folder of a film's files kept for a video outside the library is saved with its job, and with a video added without a job beside the time it was added, which an older Kando still reads; both follow the library's folder as it is renamed.
func TestHomesAreSavedAndFollowTheLibrary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")
	lib, _ := Load(path)
	root := "/films/a.kando"
	at := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	lib.Jobs = []core.Job{{ID: "j1", Source: "file:///videos/a.mp4", Stage: core.StageComplete, VideoPath: "/videos/a.mp4", LibraryDir: root, Home: root + "/a [j1]"}}
	lib.AddVideos(root, at, "/videos/b.mp4", "/videos/c.mp4")
	lib.SetAddedHome(root, "/videos/b.mp4", root+"/b [12345678]")
	if err := lib.Save(); err != nil {
		t.Fatal(err)
	}
	back, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if back.Jobs[0].Home != root+"/a [j1]" {
		t.Fatalf("the job's folder read back as %q", back.Jobs[0].Home)
	}
	added := back.AddedIn(root)
	if added["/videos/b.mp4"] != (Added{At: at, Home: root + "/b [12345678]"}) || added["/videos/c.mp4"] != (Added{At: at}) {
		t.Fatalf("the added videos read back as %+v", added)
	}
	if !back.MoveAdded(root, "/films/b.kando") || back.AddedIn("/films/b.kando")["/videos/b.mp4"].Home != filepath.Join("/films/b.kando", "b [12345678]") {
		t.Fatalf("moved, the added videos are %+v", back.Added)
	}
	if !back.MoveAddedVideo("/films/b.kando", "/films/c.kando", "/videos/b.mp4", "/films/c.kando/b [12345678]") || back.AddedIn("/films/c.kando")["/videos/b.mp4"].Home != "/films/c.kando/b [12345678]" {
		t.Fatalf("moved to another library, the added videos are %+v", back.Added)
	}
}

// The folder of a film's files is named after its video, or its video's folder for a video.<ext>, with the first 8 characters of its key.
func TestHomeFor(t *testing.T) {
	if got := HomeFor("/lib.kando", "/videos/My Talk.mp4", "0123456789abcdef"); got != filepath.Join("/lib.kando", "My Talk [01234567]") {
		t.Fatal(got)
	}
	if got := HomeFor("/lib.kando", "/videos/Course/video.mkv", "abc"); got != filepath.Join("/lib.kando", "Course [abc]") {
		t.Fatal(got)
	}
	// A name a folder on a Windows, exFAT or SMB disk cannot have is made one it can.
	if got := HomeFor("/lib.kando", `/videos/Q&A: why? <part "1"> a|b*c.mp4`, "abc"); got != filepath.Join("/lib.kando", "Q&A_ why_ _part _1__ a_b_c [abc]") {
		t.Fatal(got)
	}
	// One too long for a folder's name, with its [id], is cut where a character ends.
	long := strings.Repeat("長", 200) + ".mp4"
	got := filepath.Base(HomeFor("/lib.kando", "/videos/"+long, "0123456789abcdef"))
	if len(got) > 255 || !strings.HasSuffix(got, " [01234567]") || !utf8.ValidString(got) {
		t.Fatalf("a long name's folder is %q, %d bytes", got, len(got))
	}
	// A video named by its episode's number alone takes its folder's name before it, and a season's folder its show's, the folder's name cut short before the video's.
	for video, want := range map[string]string{
		"/videos/River Walk/01.mkv":                      "River Walk 01 [abc]",
		"/videos/[BD] River Walk/第3话.mp4":                "River Walk 第3话 [abc]",
		"/videos/River Walk/Season 2/EP05.mkv":           "River Walk Season 2 EP05 [abc]",
		"/videos/River Walk/02/video.mkv":                "River Walk 02 [abc]",
		"/videos/River Walk/01 Opening.mkv":              "01 Opening [abc]",
		"/videos/" + strings.Repeat("長", 60) + "/01.mkv": strings.Repeat("長", 39) + " 01 [abc]",
	} {
		if got := filepath.Base(HomeFor("/lib.kando", filepath.FromSlash(video), "abc")); got != want {
			t.Errorf("HomeFor(%q) = %q, want %q", video, got, want)
		}
	}
	if AddedKey("/videos/a.mp4") == AddedKey("/videos/b.mp4") || AddedKey("/videos/a.mp4") != AddedKey("/videos/./a.mp4") {
		t.Fatal("AddedKey does not tell videos apart by their cleaned paths")
	}
}
