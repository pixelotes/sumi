package media

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
)

// epub.go: minimal EPUB parser, just enough to dig out a cover image.
// EPUB is a zip with:
//   META-INF/container.xml  → points at the OPF package file
//   <package>.opf           → manifest of resources + metadata (with cover)
//
// We don't validate the spec; we extract one image and return it. Bad
// epubs fall through to "no cover" and the UI uses the generic icon.

type containerXML struct {
	Rootfiles struct {
		Rootfile []struct {
			FullPath string `xml:"full-path,attr"`
		} `xml:"rootfile"`
	} `xml:"rootfiles"`
}

type opfPackage struct {
	Metadata struct {
		Meta []struct {
			Name    string `xml:"name,attr"`
			Content string `xml:"content,attr"`
		} `xml:"meta"`
	} `xml:"metadata"`
	Manifest struct {
		Item []struct {
			ID         string `xml:"id,attr"`
			Href       string `xml:"href,attr"`
			MediaType  string `xml:"media-type,attr"`
			Properties string `xml:"properties,attr"`
		} `xml:"item"`
	} `xml:"manifest"`
}

// ExtractEpubCover opens an .epub, locates the cover image referenced in
// its OPF manifest, and returns the bytes + mime. Strategy:
//   1) META-INF/container.xml → OPF path
//   2) OPF manifest item with `properties="cover-image"` (EPUB 3) OR
//      the `<meta name="cover" content="ID">` referenced item (EPUB 2)
//   3) Fallback: first manifest item with image/* media type whose ID
//      or href contains "cover".
//
// Returns ErrNoCover if nothing matches.
func ExtractEpubCover(epubPath string) ([]byte, string, error) {
	zr, err := zip.OpenReader(epubPath)
	if err != nil {
		return nil, "", fmt.Errorf("open epub: %w", err)
	}
	defer zr.Close()

	opfPath, err := readContainer(&zr.Reader)
	if err != nil {
		return nil, "", err
	}

	pkg, err := readOPF(&zr.Reader, opfPath)
	if err != nil {
		return nil, "", err
	}

	coverHref, coverMime := selectCoverFromOPF(pkg)
	if coverHref == "" {
		return nil, "", ErrNoCover
	}

	// OPF hrefs are relative to the OPF file's directory.
	opfDir := path.Dir(opfPath)
	target := path.Clean(path.Join(opfDir, coverHref))

	for _, f := range zr.File {
		if f.Name == target {
			rc, err := f.Open()
			if err != nil {
				return nil, "", err
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, "", err
			}
			return data, coverMime, nil
		}
	}
	return nil, "", ErrNoCover
}

// ErrNoCover is returned when the epub has no extractable cover image.
var ErrNoCover = fmt.Errorf("epub: no cover image")

func readContainer(zr *zip.Reader) (string, error) {
	for _, f := range zr.File {
		if f.Name == "META-INF/container.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				return "", err
			}
			var c containerXML
			if err := xml.Unmarshal(data, &c); err != nil {
				return "", err
			}
			if len(c.Rootfiles.Rootfile) == 0 {
				return "", fmt.Errorf("container.xml: no rootfile")
			}
			return c.Rootfiles.Rootfile[0].FullPath, nil
		}
	}
	return "", fmt.Errorf("container.xml not found")
}

func readOPF(zr *zip.Reader, opfPath string) (*opfPackage, error) {
	for _, f := range zr.File {
		if f.Name == opfPath {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, err
			}
			var pkg opfPackage
			if err := xml.Unmarshal(data, &pkg); err != nil {
				return nil, err
			}
			return &pkg, nil
		}
	}
	return nil, fmt.Errorf("opf not found at %s", opfPath)
}

// selectCoverFromOPF returns href and MIME for the cover image, or "" / "".
func selectCoverFromOPF(pkg *opfPackage) (string, string) {
	// EPUB 3 — properties="cover-image"
	for _, item := range pkg.Manifest.Item {
		if strings.Contains(item.Properties, "cover-image") &&
			strings.HasPrefix(item.MediaType, "image/") {
			return item.Href, item.MediaType
		}
	}

	// EPUB 2 — <meta name="cover" content="..."> referencing a manifest id.
	for _, meta := range pkg.Metadata.Meta {
		if strings.EqualFold(meta.Name, "cover") && meta.Content != "" {
			for _, item := range pkg.Manifest.Item {
				if item.ID == meta.Content && strings.HasPrefix(item.MediaType, "image/") {
					return item.Href, item.MediaType
				}
			}
		}
	}

	// Heuristic — first image whose id/href contains "cover".
	for _, item := range pkg.Manifest.Item {
		if !strings.HasPrefix(item.MediaType, "image/") {
			continue
		}
		hay := strings.ToLower(item.ID + " " + item.Href)
		if strings.Contains(hay, "cover") {
			return item.Href, item.MediaType
		}
	}

	return "", ""
}

// IsEpubExt is a small shortcut for callers that have a path.
func IsEpubExt(p string) bool {
	return strings.EqualFold(filepath.Ext(p), ".epub")
}
