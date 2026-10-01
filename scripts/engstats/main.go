// Command engstats aggregates the English error log into a ranked summary.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"engimprove/internal/data"
	"engimprove/internal/stats"
)

type options struct {
	in       string
	out      string
	taxonomy string
	top      int
	since    string
	kind     string
	write    bool
	noColor  bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "engstats: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}

	var opt options
	flag.StringVar(&opt.in, "in", filepath.Join(root, "errors", "errors.jsonl"), "path to the error log")
	flag.StringVar(&opt.out, "out", filepath.Join(root, "errors", "stats.md"), "path of the generated summary")
	flag.StringVar(&opt.taxonomy, "taxonomy", filepath.Join(root, "errors", "taxonomy.md"), "path to the category taxonomy")
	flag.IntVar(&opt.top, "top", 10, "how many rows per ranking")
	flag.StringVar(&opt.since, "since", "", "only count entries on or after this date (YYYY-MM-DD)")
	flag.StringVar(&opt.kind, "kind", "", "only count this kind (grammar, spelling, punctuation, lexical, style)")
	flag.BoolVar(&opt.write, "write", false, "regenerate the summary file as well")
	flag.BoolVar(&opt.noColor, "no-color", false, "disable ANSI colors")
	flag.Parse()

	if opt.top < 1 {
		return fmt.Errorf("-top must be >= 1, got %d", opt.top)
	}
	if opt.since != "" {
		if _, err := time.Parse("2006-01-02", opt.since); err != nil {
			return fmt.Errorf("-since must be YYYY-MM-DD, got %q", opt.since)
		}
	}

	entries, err := load(opt.in)
	if err != nil {
		return err
	}
	entries = filter(entries, opt)
	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, stamp(), "no entries matched; nothing to report")
	}

	warnUnknownCategories(entries, opt.taxonomy)

	fmt.Print(stats.Render(entries, opt.top, useColor(opt.noColor)))

	if opt.write {
		if err := os.WriteFile(opt.out, []byte(stats.Markdown(entries, opt.top, readLanguage(filepath.Join(root, "config", "eng.json")))), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", opt.out, err)
		}
		fmt.Fprintln(os.Stderr, stamp(), "wrote", opt.out)
	}
	return nil
}

// readLanguage takes the learner's language from the config; English when absent.
func readLanguage(path string) string {
	var cfg struct {
		Language string `json:"language"`
	}
	if err := data.ReadJSON(path, &cfg); err != nil || cfg.Language == "" {
		return "English"
	}
	return cfg.Language
}

// moduleRoot walks up from the cwd so the tool works from any subdirectory.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found in any parent directory")
		}
		dir = parent
	}
}

func load(path string) ([]data.Entry, error) {
	entries, err := data.ReadJSONL[data.Entry](path)
	if err != nil {
		return nil, err
	}
	for i, e := range entries {
		if e.Date == "" || e.Category == "" || e.Kind == "" {
			return nil, fmt.Errorf("%s: entry %d: date, kind and category are required", path, i+1)
		}
		if _, err := time.Parse("2006-01-02", e.Date); err != nil {
			return nil, fmt.Errorf("%s: entry %d: bad date %q", path, i+1, e.Date)
		}
	}
	return entries, nil
}

func filter(in []data.Entry, opt options) []data.Entry {
	var out []data.Entry
	for _, e := range in {
		if opt.since != "" && e.Date < opt.since {
			continue
		}
		if opt.kind != "" && e.Kind != opt.kind {
			continue
		}
		out = append(out, e)
	}
	return out
}

var taxonomyCategory = regexp.MustCompile("^\\|\\s*`([a-z-]+)`")

func warnUnknownCategories(entries []data.Entry, taxonomyPath string) {
	raw, err := os.ReadFile(taxonomyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, stamp(), "taxonomy unreadable, skipping category check:", err)
		return
	}
	known := map[string]bool{}
	for line := range strings.SplitSeq(string(raw), "\n") {
		if m := taxonomyCategory.FindStringSubmatch(line); m != nil {
			known[m[1]] = true
		}
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if !known[e.Category] && !seen[e.Category] {
			seen[e.Category] = true
			fmt.Fprintf(os.Stderr, "%s category %q is not in taxonomy.md\n", stamp(), e.Category)
		}
	}
}

func useColor(noColor bool) bool {
	if noColor || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func stamp() string {
	return time.Now().Format("15:04:05")
}
