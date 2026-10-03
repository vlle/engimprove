// Package resume loads the learner's resume and STAR stories for context-aware reviews.
package resume

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Loader reads and caches resume files from a directory.
type Loader struct {
	dir    string
	mu     sync.RWMutex
	cached string
	mtimes map[string]timeWrap
}

type timeWrap struct{ mtime int64 }

// NewLoader creates a loader for dir. An empty dir disables loading.
func NewLoader(dir string) *Loader {
	return &Loader{dir: dir, mtimes: map[string]timeWrap{}}
}

// Enabled reports whether a resume directory is configured.
func (l *Loader) Enabled() bool { return l.dir != "" }

// Context returns the concatenated resume text, reloading if any file changed.
func (l *Loader) Context() string {
	if !l.Enabled() {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cached != "" && !l.changed() {
		return l.cached
	}
	l.cached = l.load()
	return l.cached
}

// changed reports whether any watched file has a different mtime.
func (l *Loader) changed() bool {
	for path, tw := range l.mtimes {
		info, err := os.Stat(path)
		if err != nil || info.ModTime().Unix() != tw.mtime {
			return true
		}
	}
	return false
}

// load reads the configured files and records mtimes.
func (l *Loader) load() string {
	files := []string{
		"resume.txt",
		"hunt/stories.md",
		"cover-letter.md",
	}
	var parts []string
	l.mtimes = map[string]timeWrap{}
	for _, name := range files {
		path := filepath.Join(l.dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		info, _ := os.Stat(path)
		if info != nil {
			l.mtimes[path] = timeWrap{mtime: info.ModTime().Unix()}
		}
		parts = append(parts, "--- "+name+" ---\n"+string(data))
	}
	return strings.Join(parts, "\n\n")
}

// Files returns the resume files that exist, sorted.
func (l *Loader) Files() []string {
	if !l.Enabled() {
		return nil
	}
	var out []string
	for _, name := range []string{"resume.txt", "hunt/stories.md", "cover-letter.md"} {
		if _, err := os.Stat(filepath.Join(l.dir, name)); err == nil {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
