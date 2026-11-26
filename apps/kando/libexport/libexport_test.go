package libexport

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"kando/apps/kando/subexport"
	"kando/packages/core"
	"kando/packages/media"
)

// An export is named in the language slot players read: the target's for both lines or the translation, the spoken one's for the original, Simplified Chinese as zh and Traditional as zh-Hant; a film of no known language takes the 导出字幕 dialog's name.
func TestName(t *testing.T) {
	c := subexport.Default()
	for _, tc := range []struct {
		content, target, spoken, want string
	}{
		{core.SubBilingual, "简体中文", "ja", "ep 1.zh.srt"},
		{core.SubTranslation, "繁体中文", "ja", "ep 1.zh-Hant.srt"},
		{core.SubOriginal, "简体中文", "ja", "ep 1.ja.srt"},
		{core.SubTranslation, "English", "ko", "ep 1.en.srt"},
		{core.SubBilingual, "", "", "ep 1.dual.srt"},
		{core.SubOriginal, "简体中文", "", "ep 1.original.srt"},
	} {
		c.Content = tc.content
		if got := Name("/v/ep 1.mkv", c, tc.target, tc.spoken, ".srt"); got != tc.want {
			t.Errorf("%s of %q spoken %q is %q, want %q", tc.content, tc.target, tc.spoken, got, tc.want)
		}
	}
}

// An export writes a file that is not there, leaves its own that holds the same as it is, writes its own over once it changed, and never writes over a file of the user's under its name, nor over its own once the user changed it.
func TestWriteKeepsTheUsersFiles(t *testing.T) {
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, media.FilesDir, LedgerName)
	path := filepath.Join(dir, "ep.zh.srt")
	l := LoadLedger(ledgerPath)
	if got, err := Write(path, "ep.zh.srt", []byte("one"), l); err != nil || got != Written {
		t.Fatalf("a new file: %v %v", got, err)
	}
	if err := l.Save(ledgerPath); err != nil {
		t.Fatal(err)
	}
	l = LoadLedger(ledgerPath)
	if got, _ := Write(path, "ep.zh.srt", []byte("one"), l); got != Same {
		t.Fatalf("the same again: %v", got)
	}
	if got, _ := Write(path, "ep.zh.srt", []byte("two"), l); got != Written {
		t.Fatalf("its own changed: %v", got)
	}
	if err := os.WriteFile(path, []byte("the user's"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := Write(path, "ep.zh.srt", []byte("three"), l); got != Kept {
		t.Fatalf("its own the user changed: %v", got)
	}
	users := filepath.Join(dir, "other.zh.srt")
	if err := os.WriteFile(users, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := Write(users, "other.zh.srt", []byte("export"), l); got != Kept {
		t.Fatalf("the user's file: %v", got)
	}
	if data, _ := os.ReadFile(users); string(data) != "mine" {
		t.Fatalf("the user's file holds %q", data)
	}
}

// What a Kando before wrote beside a video that has its folder now is offered to go, but for the transcript under the video's own name, which may be the user's; a film still reading its files beside its video has none.
func TestOldBeside(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "ep.mp4")
	for _, path := range []string{video, media.BilingualPath(video), filepath.Join(dir, "ep.srt"), media.CorrectionsPath(video)} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := OldBeside(video, ""); got != nil {
		t.Fatalf("a film with no folder of its own: %v", got)
	}
	home := media.FolderHome(video)
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	got := OldBeside(video, home)
	if !slices.Contains(got, media.BilingualPath(video)) || !slices.Contains(got, media.CorrectionsPath(video)) || slices.Contains(got, filepath.Join(dir, "ep.srt")) {
		t.Fatalf("old beside: %v", got)
	}
}
