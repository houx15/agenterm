package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type fsDirectoryEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type fsDirectoryResponse struct {
	Path        string             `json:"path"`
	Parent      string             `json:"parent,omitempty"`
	Directories []fsDirectoryEntry `json:"directories"`
}

func (h *handler) listDirectories(w http.ResponseWriter, r *http.Request) {
	targetPath, err := normalizeBrowsePath(r.URL.Query().Get("path"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid path")
		return
	}
	info, err := os.Stat(targetPath)
	if err != nil || !info.IsDir() {
		jsonError(w, http.StatusBadRequest, "path must be an existing directory")
		return
	}

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "failed to read directory")
		return
	}

	dirs := make([]fsDirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		dirs = append(dirs, fsDirectoryEntry{
			Name: name,
			Path: filepath.Join(targetPath, name),
		})
	}

	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})

	parent := filepath.Dir(targetPath)
	if parent == targetPath {
		parent = ""
	}

	jsonResponse(w, http.StatusOK, fsDirectoryResponse{
		Path:        targetPath,
		Parent:      parent,
		Directories: dirs,
	})
}

func normalizeBrowsePath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Clean(home), nil
	}

	if strings.HasPrefix(trimmed, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if trimmed == "~" {
			trimmed = home
		} else {
			trimmed = filepath.Join(home, strings.TrimPrefix(trimmed, "~/"))
		}
	}

	if !filepath.IsAbs(trimmed) {
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			return "", err
		}
		trimmed = abs
	}

	return filepath.Clean(trimmed), nil
}
