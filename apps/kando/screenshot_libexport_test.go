package main

import (
	"testing"

	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestScreenshotsLibexport renders the sample library with the 导出资料库的字幕 dialog open over it (文件 › 导出资料库的字幕…), as it opens.
func TestScreenshotsLibexport(t *testing.T) {
	f := newScreenshotFixture(t)
	f.shoot(t, "libexport-dialog", func(t *testing.T, a *app, tt *nsgotest.App, updates chan func()) {
		a.OpenLibexport()
		refresh(tt, a)
		if !tt.Exists("id:libexport-dialog") {
			t.Fatalf("the dialog did not open: %q", tt.Texts())
		}
	})
}
