// Package buildinfo reports the binary's version from runtime build metadata.
package buildinfo

import (
	"runtime/debug"
	"strings"
	"time"
)

// Info holds a short version string for the UI.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Time    string `json:"time"`
	Dirty   bool   `json:"dirty"`
}

// Read extracts VCS info from the embedded build data.
func Read() Info {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return Info{Version: "dev"}
	}
	out := Info{Version: bi.Main.Version}
	if out.Version == "" || out.Version == "(devel)" {
		out.Version = "dev"
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			out.Commit = s.Value
			if len(out.Commit) > 7 {
				out.Commit = out.Commit[:7]
			}
		case "vcs.time":
			if t, err := time.Parse(time.RFC3339, s.Value); err == nil {
				out.Time = t.Format("2006-01-02 15:04")
			}
		case "vcs.modified":
			out.Dirty = s.Value == "true"
		}
	}
	return out
}

// String returns a concise version label.
func (i Info) String() string {
	parts := []string{i.Version}
	if i.Commit != "" {
		parts = append(parts, i.Commit)
	}
	if i.Time != "" {
		parts = append(parts, i.Time)
	}
	if i.Dirty {
		parts = append(parts, "dirty")
	}
	return strings.Join(parts, " · ")
}
