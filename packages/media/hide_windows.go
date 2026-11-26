package media

import "syscall"

// hideFolder marks the folder at path hidden, so File Explorer does not show it, as a name starting with a dot does not there: Plex and Jellyfin skip it by its dot on every system. A folder it cannot mark is left as it is.
func hideFolder(path string) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	attrs, err := syscall.GetFileAttributes(name)
	if err != nil || attrs&syscall.FILE_ATTRIBUTE_HIDDEN != 0 {
		return
	}
	_ = syscall.SetFileAttributes(name, attrs|syscall.FILE_ATTRIBUTE_HIDDEN)
}
