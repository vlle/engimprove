// Package data reads and writes the engimprove files: the error log, state and config.
package data

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Entry is one mistake from errors/errors.jsonl.
type Entry struct {
	ID       string `json:"id"`
	Date     string `json:"date"`
	TextID   string `json:"text_id"`
	Kind     string `json:"kind"`
	Category string `json:"category"`
	Rule     string `json:"rule"`
	Before   string `json:"before"`
	After    string `json:"after"`
	Note     string `json:"note"`
}

// Store resolves every engimprove path against one root directory.
type Store struct {
	Root string
}

// Open finds the engimprove root: $ENG_ROOT, the binary's parent dir, or a parent of the cwd.
func Open() (Store, error) {
	if root := os.Getenv("ENG_ROOT"); root != "" {
		if !isRoot(root) {
			return Store{}, fmt.Errorf("ENG_ROOT=%s is not an engimprove checkout", root)
		}
		return Store{Root: root}, nil
	}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		if dir := filepath.Dir(filepath.Dir(exe)); isRoot(dir) {
			return Store{Root: dir}, nil
		}
	}
	dir, err := os.Getwd()
	if err != nil {
		return Store{}, err
	}
	for {
		if isRoot(dir) {
			return Store{Root: dir}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Store{}, errors.New("engimprove root not found: set ENG_ROOT or run inside the checkout")
		}
		dir = parent
	}
}

func isRoot(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "errors", "errors.jsonl")); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}

// Path joins parts onto the root.
func (s Store) Path(parts ...string) string {
	return filepath.Join(append([]string{s.Root}, parts...)...)
}

// Entries loads the error log in file order.
func (s Store) Entries() ([]Entry, error) {
	entries, err := ReadJSONL[Entry](s.Path("errors", "errors.jsonl"))
	if err != nil {
		return nil, err
	}
	for i, e := range entries {
		if e.ID == "" || e.Date == "" || e.Category == "" || e.Kind == "" {
			return nil, fmt.Errorf("errors.jsonl entry %d: id, date, kind and category are required", i+1)
		}
	}
	return entries, nil
}

// Categories returns the closed category list from errors/taxonomy.md, keyed to its kind.
func (s Store) Categories() (map[string]string, error) {
	raw, err := os.ReadFile(s.Path("errors", "taxonomy.md"))
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	kind := ""
	for line := range strings.SplitSeq(string(raw), "\n") {
		if rest, ok := strings.CutPrefix(line, "## kind: "); ok {
			kind, _, _ = strings.Cut(rest, " ")
			continue
		}
		if !strings.HasPrefix(line, "| `") || kind == "" {
			continue
		}
		name, _, ok := strings.Cut(strings.TrimPrefix(line, "| `"), "`")
		if ok {
			out[name] = kind
		}
	}
	if len(out) == 0 {
		return nil, errors.New("taxonomy.md has no categories")
	}
	return out, nil
}

// ReadJSONL decodes one value per non-empty line; a missing file is empty.
func ReadJSONL[T any](path string) ([]T, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []T
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var v T
		if err := json.Unmarshal([]byte(text), &v); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		out = append(out, v)
	}
	return out, sc.Err()
}

// AppendJSONL appends values as compact JSON lines.
func AppendJSONL[T any](path string, values ...T) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	for _, v := range values {
		line, err := json.Marshal(v)
		if err != nil {
			return err
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// ReadJSON decodes a file into v; a missing file leaves v untouched.
func ReadJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// WriteJSON replaces a file atomically so readers never see half a document.
func WriteJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return WriteFile(path, append(raw, '\n'))
}

// WriteFile writes through a temp file and rename.
func WriteFile(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Locked runs fn while holding an exclusive lock: hook, server and cli share state files.
func (s Store) Locked(name string, fn func() error) error {
	path := s.Path("state", "."+name+".lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}

// NextIDs returns n fresh ids for date, continuing that day's numbering.
func NextIDs(entries []Entry, date string, n int) []string {
	last := 0
	for _, e := range entries {
		rest, ok := strings.CutPrefix(e.ID, date+"-")
		if !ok {
			continue
		}
		var num int
		if _, err := fmt.Sscanf(rest, "%d", &num); err == nil && num > last {
			last = num
		}
	}
	ids := make([]string, n)
	for i := range n {
		ids[i] = fmt.Sprintf("%s-%03d", date, last+i+1)
	}
	return ids
}

// Today is the local date in ISO form.
func Today() string {
	return time.Now().Format("2006-01-02")
}

// RuleCounts counts entries per rule.
func RuleCounts(entries []Entry) map[string]int {
	out := map[string]int{}
	for _, e := range entries {
		out[e.Rule]++
	}
	return out
}

// SortNewestFirst orders entries by id descending; ids embed the date.
func SortNewestFirst(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].ID > entries[j].ID })
}
