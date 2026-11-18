package theme

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// surfaceLiteral is a drop shadow in a black written out, or the dark glass's fill written out: what the design's shadows and glasses (surface.go) give.
var surfaceLiteral = regexp.MustCompile(`Blur: [0-9.]+, Color: (core|ns)\.RGBA\(0x000000|RGBA\(0x1C1C20|RGBA\(0x1E1E22`)

// TestSurfacesComeFromTheTheme checks that the app's code outside this package takes its drop shadows and its dark glass from the design's (Shadow…, Glass…), so surfaces of a kind look alike and change together, rather than each writing its own.
func TestSurfacesComeFromTheTheme(t *testing.T) {
	app := ".."
	if _, err := os.Stat(filepath.Join(app, "main.go")); err != nil {
		t.Skip("the app's sources are not beside the theme package here")
	}
	err := filepath.WalkDir(app, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != app && (name == "theme" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			if surfaceLiteral.MatchString(line) {
				t.Errorf("%s:%d writes out a shadow or a glass the theme has (theme.Shadow…, theme.Glass…): %s", path, i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
