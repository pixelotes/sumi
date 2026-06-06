package media

import (
	"os"
	"path/filepath"
	"strings"
)

// FindFolderCover returns a cover image alongside a file (cover.jpg /
// folder.jpg / book.jpg), or "" if there isn't one. Used for directory
// posters and as a per-book override.
func FindFolderCover(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	priorities := []string{"cover", "folder", "book", "front"}
	for _, prefix := range priorities {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := strings.ToLower(entry.Name())
			ext := filepath.Ext(name)
			base := strings.TrimSuffix(name, ext)
			if base == prefix {
				if _, ok := imageExts[ext]; ok {
					return filepath.Join(dir, entry.Name())
				}
			}
		}
	}
	return ""
}

// FindBookCover looks for a cover image with the same basename as the
// book ("My Book.epub" → "My Book.jpg"). Falls back to the folder cover.
func FindBookCover(bookPath string) string {
	dir := filepath.Dir(bookPath)
	baseNoExt := strings.TrimSuffix(filepath.Base(bookPath), filepath.Ext(bookPath))
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp"} {
		candidate := filepath.Join(dir, baseNoExt+ext)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return FindFolderCover(dir)
}

// CoverContentType is the MIME for a (folder or extracted) cover.
func CoverContentType(path string) string {
	return imageContentType(path)
}
