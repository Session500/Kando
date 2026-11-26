//go:build !windows

package media

// hideFolder does nothing where a name starting with a dot hides a folder already.
func hideFolder(string) {}
