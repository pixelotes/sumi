package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sumi/internal/media"
)

type browseItem struct {
	Name         string `json:"name"`
	FriendlyName string `json:"friendly_name,omitempty"`
	Path         string `json:"path"`
	IsDir        bool   `json:"is_dir"`
	IsBook       bool   `json:"is_book,omitempty"`
	Icon         string `json:"icon,omitempty"`
	StreamURL    string `json:"stream_url,omitempty"`
}

type folderInfo struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Poster string `json:"poster,omitempty"`
	Parent string `json:"parent,omitempty"`
}

type browseResponse struct {
	Items         []browseItem `json:"items"`
	CurrentFolder *folderInfo  `json:"current_folder,omitempty"`
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" || path == "." {
		var items []browseItem
		for _, lib := range s.cfg.Libraries {
			item := browseItem{
				Name:         filepath.Base(lib.Path),
				FriendlyName: lib.FriendlyName,
				Path:         lib.Path,
				IsDir:        true,
			}
			if cover := media.FindFolderCover(lib.Path); cover != "" {
				item.Icon = "/api/v1/image?path=" + urlEscape(cover)
			}
			items = append(items, item)
		}
		writeJSON(w, browseResponse{Items: items})
		return
	}

	if !s.isPathAllowed(path) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	resp, err := s.browseDir(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, resp)
}

func (s *Server) browseDir(dirPath string) (*browseResponse, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var items []browseItem
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(dirPath, name)
		if e.IsDir() {
			item := browseItem{Name: name, Path: full, IsDir: true}
			if cover := media.FindFolderCover(full); cover != "" {
				item.Icon = "/api/v1/image?path=" + urlEscape(cover)
			}
			items = append(items, item)
			continue
		}
		if media.IsEbook(name) {
			items = append(items, browseItem{
				Name:      name,
				Path:      full,
				IsDir:     false,
				IsBook:    true,
				Icon:      "/api/v1/cover?path=" + urlEscape(full),
				StreamURL: "/api/v1/stream?path=" + urlEscape(full),
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return naturalLessASCII(items[i].Name, items[j].Name)
	})

	folder := &folderInfo{Name: filepath.Base(dirPath), Path: dirPath}
	if cover := media.FindFolderCover(dirPath); cover != "" {
		folder.Poster = "/api/v1/image?path=" + urlEscape(cover)
	}
	parent := filepath.Dir(dirPath)
	if parent != dirPath && s.isPathAllowed(parent) {
		folder.Parent = parent
	}

	return &browseResponse{Items: items, CurrentFolder: folder}, nil
}

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if !s.isPathAllowed(path) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, path)
}

// handleCover serves a per-book cover. Priority:
//   1) A sibling image file ("My Book.jpg" next to "My Book.epub").
//   2) A folder-wide cover (cover.jpg / folder.jpg / book.jpg).
//   3) The cover image embedded inside the epub.
// Anything else → 404.
func (s *Server) handleCover(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if !s.isPathAllowed(path) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !media.IsEbook(path) {
		http.NotFound(w, r)
		return
	}

	if sidecar := media.FindBookCover(path); sidecar != "" {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Content-Type", media.CoverContentType(sidecar))
		http.ServeFile(w, r, sidecar)
		return
	}

	data, mime, err := media.ExtractEpubCover(path)
	if err != nil {
		if errors.Is(err, media.ErrNoCover) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to extract cover", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", mime)
	_, _ = w.Write(data)
}

// handleStream serves the raw .epub file (Range-supported by ServeFile).
// epub.js consumes it directly client-side.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if !s.isPathAllowed(path) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !media.IsEbook(path) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", "application/epub+zip")
	http.ServeFile(w, r, path)
}

// --- Path guard / helpers ---

func (s *Server) isPathAllowed(p string) bool {
	abs, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	for _, lib := range s.cfg.Libraries {
		rel, err := filepath.Rel(lib.Path, abs)
		if err != nil {
			continue
		}
		if rel == "." || (rel != "" && !strings.HasPrefix(rel, "..")) {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func urlEscape(s string) string {
	r := strings.NewReplacer(
		"%", "%25",
		" ", "%20",
		"?", "%3F",
		"#", "%23",
		"&", "%26",
		"+", "%2B",
	)
	return r.Replace(s)
}

func naturalLessASCII(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	ia, ib := 0, 0
	for ia < len(la) && ib < len(lb) {
		ca, cb := la[ia], lb[ib]
		if ca >= '0' && ca <= '9' && cb >= '0' && cb <= '9' {
			na, nb := 0, 0
			for ia < len(la) && la[ia] >= '0' && la[ia] <= '9' {
				na = na*10 + int(la[ia]-'0')
				ia++
			}
			for ib < len(lb) && lb[ib] >= '0' && lb[ib] <= '9' {
				nb = nb*10 + int(lb[ib]-'0')
				ib++
			}
			if na != nb {
				return na < nb
			}
		} else {
			if ca != cb {
				return ca < cb
			}
			ia++
			ib++
		}
	}
	return len(la) < len(lb)
}
