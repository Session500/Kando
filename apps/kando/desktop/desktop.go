// Package desktop opens URLs, reveals and trashes files, and keeps the computer awake for Kando, through MyGo's Shell and Power. Notifications, the Dock badge and the progress bar are not here: they stay with the queue in queue_desktop.go.
package desktop

import "github.com/egoist/mygo"

// OpenURL opens url with the system's default app for it: a web page in the browser, or a pane of System Settings for an x-apple.systempreferences: URL.
func OpenURL(url string) error { return mygo.Shell.OpenExternal(url) }

// Reveal shows path selected in the file manager: Finder selecting the item on macOS, File Explorer selecting it on Windows, and on Linux the file manager's org.freedesktop.FileManager1 ShowItems, which selects it too, falling back to opening its folder when no file manager answers. It reports no failure, as each system's reveal either shows the item or falls back on its own.
func Reveal(path string) { mygo.Shell.ShowItemInFolder(path) }

// Trash moves a file or a folder to the Trash, the Recycle Bin on Windows. It fails, and leaves the item where it is, when the item cannot go there and would be deleted for good instead, as on a network share on Windows.
func Trash(path string) error { return mygo.Shell.TrashItem(path) }

// KeepAwake keeps the computer from sleeping, and with display set the display from turning off too, until release is called; calling release again does nothing. The reason shows where the system lists what keeps it awake. It needs the running app, so tests must not call it.
func KeepAwake(reason string, display bool) (release func()) {
	return mygo.Power.KeepAwake(reason, display)
}
