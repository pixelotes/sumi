package media

import (
	"encoding/base64"
	"path/filepath"
	"strings"
)

// EncodePath turns an absolute filesystem path into an opaque URL-safe ID.
func EncodePath(path string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(path))
}

// DecodePath inverts EncodePath.
func DecodePath(encoded string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

var ebookExts = map[string]bool{
	".epub": true,
}

var imageExts = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
	".gif":  "image/gif",
}

// IsEbook reports whether a filename has a supported ebook extension.
// Today only EPUB is fully supported; PDF/MOBI/AZW3 would need their own
// viewers and aren't worth the weight on a Pi yet.
func IsEbook(name string) bool {
	return ebookExts[strings.ToLower(filepath.Ext(name))]
}

func imageContentType(path string) string {
	if ct, ok := imageExts[strings.ToLower(filepath.Ext(path))]; ok {
		return ct
	}
	return "application/octet-stream"
}
