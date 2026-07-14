package app

// OpenURL opens a URL with the system default handler.
func OpenURL(url string) error {
	return openURL(url)
}

// OpenPath opens a file or folder with the system default handler.
func OpenPath(path string) error {
	return openPath(path)
}

// OpenInNotepad opens a file in Notepad (Windows) or a plain-text editor.
func OpenInNotepad(path string) error {
	return openInNotepad(path)
}
