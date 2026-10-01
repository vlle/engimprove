// Package topics decides which drills are ripe from the mistakes logged since the last drill.
package topics

import (
	"slices"
	"sort"

	"engimprove/internal/config"
	"engimprove/internal/data"
	"engimprove/internal/diff"
)

// Session is one finished drill; Cursor is the newest mistake it covered.
type Session struct {
	TS      string `json:"ts"`
	Topic   string `json:"topic"`
	Cursor  string `json:"cursor"`
	Asked   int    `json:"asked"`
	Correct int    `json:"correct"`
}

// RuleCount is how often one rule shows up inside a topic.
type RuleCount struct {
	Rule     string `json:"rule"`
	Category string `json:"category"`
	Count    int    `json:"count"`
	Fresh    int    `json:"fresh"`
	Last     string `json:"last"`
}

// Status is the ripeness of one topic.
type Status struct {
	ID        string      `json:"id"`
	Title     string      `json:"title"`
	Threshold int         `json:"threshold"`
	Total     int         `json:"total"`
	Fresh     int         `json:"fresh"`
	Ready     bool        `json:"ready"`
	Cursor    string      `json:"cursor"`
	LastDrill string      `json:"last_drill"`
	Drills    int         `json:"drills"`
	Accuracy  float64     `json:"accuracy"`
	Rules     []RuleCount `json:"rules"`
}

// Matches reports whether a mistake belongs to the topic.
func Matches(t config.Topic, e data.Entry) bool {
	if !slices.Contains(t.Categories, e.Category) {
		return false
	}
	return diff.Touches(e.Before, e.After, t.Tokens)
}

// Of returns the ids of every topic the mistake belongs to.
func Of(cfg config.Config, e data.Entry) []string {
	var out []string
	for _, t := range cfg.Topics {
		if Matches(t, e) {
			out = append(out, t.ID)
		}
	}
	return out
}

// Sessions loads the drill history.
func Sessions(s data.Store) ([]Session, error) {
	return data.ReadJSONL[Session](s.Path("state", "sessions.jsonl"))
}

// Statuses computes every topic's ripeness, ready ones first.
func Statuses(cfg config.Config, entries []data.Entry, sessions []Session) []Status {
	out := make([]Status, 0, len(cfg.Topics))
	for _, t := range cfg.Topics {
		st := Status{ID: t.ID, Title: t.Title, Threshold: t.Threshold}
		asked, correct := 0, 0
		for _, s := range sessions {
			if s.Topic != t.ID {
				continue
			}
			st.Drills++
			asked += s.Asked
			correct += s.Correct
			if s.Cursor > st.Cursor {
				st.Cursor = s.Cursor
			}
			if s.TS > st.LastDrill {
				st.LastDrill = s.TS
			}
		}
		if asked > 0 {
			st.Accuracy = float64(correct) / float64(asked)
		}
		rules := map[string]*RuleCount{}
		for _, e := range entries {
			if !Matches(t, e) {
				continue
			}
			st.Total++
			fresh := e.ID > st.Cursor
			if fresh {
				st.Fresh++
			}
			rc := rules[e.Rule]
			if rc == nil {
				rc = &RuleCount{Rule: e.Rule, Category: e.Category}
				rules[e.Rule] = rc
			}
			rc.Count++
			if fresh {
				rc.Fresh++
			}
			if e.Date > rc.Last {
				rc.Last = e.Date
			}
		}
		st.Ready = st.Fresh >= t.Threshold
		for _, rc := range rules {
			st.Rules = append(st.Rules, *rc)
		}
		sort.Slice(st.Rules, func(i, j int) bool {
			a, b := st.Rules[i], st.Rules[j]
			if a.Fresh != b.Fresh {
				return a.Fresh > b.Fresh
			}
			if a.Count != b.Count {
				return a.Count > b.Count
			}
			return a.Rule < b.Rule
		})
		out = append(out, st)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Ready != out[j].Ready {
			return out[i].Ready
		}
		return ratio(out[i]) > ratio(out[j])
	})
	return out
}

func ratio(s Status) float64 {
	return float64(s.Fresh) / float64(max(s.Threshold, 1))
}

// Cursor is the newest mistake id inside the topic.
func Cursor(t config.Topic, entries []data.Entry) string {
	cur := ""
	for _, e := range entries {
		if e.ID > cur && Matches(t, e) {
			cur = e.ID
		}
	}
	return cur
}
