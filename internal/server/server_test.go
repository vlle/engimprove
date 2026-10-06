package server

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"engimprove/internal/cards"
	"engimprove/internal/coach"
	"engimprove/internal/config"
	"engimprove/internal/data"
	"engimprove/internal/hook"
	"engimprove/internal/logbook"
	"engimprove/internal/speak"
	"engimprove/internal/testutil"
)

func TestWeekSpend(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC) // a wednesday
	tests := []struct {
		name   string
		checks []hook.CheckLog
		recs   []speak.Recording
		want   float64
	}{
		{"nothing", nil, nil, 0},
		{
			"only this week counts",
			[]hook.CheckLog{{TS: "2026-10-06T09:00:00Z", Cost: 0.01}, {TS: "2026-10-04T09:00:00Z", Cost: 0.5}},
			[]speak.Recording{{TS: "2026-10-07T08:00:00Z", WhisperCost: 0.002, ReviewCost: 0.003}},
			0.015,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := weekSpend(tt.checks, tt.recs, now); math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("spend = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRuleHistory(t *testing.T) {
	const rule = "missing definite article before a known referent"
	slip := func(id, date, rule string) data.Entry {
		return data.Entry{ID: id, Date: date, Rule: rule, Before: "opened PR " + id, After: "opened the PR " + id}
	}
	many := []data.Entry{}
	for k := 1; k <= 6; k++ {
		many = append(many, slip(fmt.Sprint(k), fmt.Sprintf("2026-09-0%d", k), rule))
	}
	tests := []struct {
		name    string
		entries []data.Entry
		card    cards.Card
		count   int
		dates   []string
	}{
		{
			name:    "own entry counted but not listed",
			entries: []data.Entry{slip("1", "2026-09-01", rule), slip("2", "2026-09-02", "other rule"), slip("3", "2026-09-03", rule)},
			card:    cards.Card{EntryID: "3", Rule: rule},
			count:   2,
			dates:   []string{"2026-09-01"},
		},
		{
			name:    "rule match ignores case",
			entries: []data.Entry{slip("1", "2026-09-01", "Missing Definite Article Before A Known Referent")},
			card:    cards.Card{Rule: rule},
			count:   1,
			dates:   []string{"2026-09-01"},
		},
		{
			name:    "newest four first",
			entries: many,
			card:    cards.Card{EntryID: "6", Rule: rule},
			count:   6,
			dates:   []string{"2026-09-05", "2026-09-04", "2026-09-03", "2026-09-02"},
		},
		{
			name:    "slip without a fix is counted only",
			entries: []data.Entry{{ID: "1", Date: "2026-09-01", Rule: rule}},
			card:    cards.Card{Rule: rule},
			count:   1,
			dates:   []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ruleHistory(tt.entries, tt.card)
			dates := []string{}
			for _, s := range got.Slips {
				dates = append(dates, s.Date)
			}
			if got.Count != tt.count || !reflect.DeepEqual(dates, tt.dates) {
				t.Errorf("got count %d dates %v, want %d %v", got.Count, dates, tt.count, tt.dates)
			}
		})
	}
}

func TestLogCheckedText(t *testing.T) {
	st, _ := testutil.Store(t)
	s := &Server{Store: st, Static: testFS{}, Coach: coach.New(config.Config{Language: "English"}), Log: discard()}
	h := s.Handler()
	post := func(id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/check/"+id+"/log", nil)
		req.Host = "127.0.0.1:7421"
		req.Header.Set("X-Eng", "1")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	draft := checkedText{
		ID: "1", Original: "I opened PR yesterday",
		Corrected: "I opened the PR yesterday",
		Errors: []logbook.NewError{
			{Kind: "grammar", Category: "articles", Rule: "missing definite article before a known referent", Before: "opened PR", After: "opened the PR"},
			{Category: "style", Rule: "wordy phrase", Before: "in order to", After: "to"},
		},
	}
	draft.TextID = "2026-10-02-web-test"
	if err := data.WriteJSON(st.Path("state", "check-web", draft.ID+".json"), draft); err != nil {
		t.Fatal(err)
	}
	if rec := post("1"); rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	entries, err := st.Entries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Category != "articles" || entries[0].TextID != draft.TextID {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	// style stays out of the database.
	if rec := post("1"); rec.Code != http.StatusConflict {
		t.Fatalf("re-log status %d, want 409", rec.Code)
	}
	if rec := post("404"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown id status %d, want 404", rec.Code)
	}
}

type testFS struct{}

func (testFS) Open(string) (fs.File, error) { return nil, fs.ErrNotExist }

func discard() *log.Logger { return log.New(io.Discard, "", 0) }
