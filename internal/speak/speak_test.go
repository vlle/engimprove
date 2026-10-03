package speak

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"engimprove/internal/coach"
	"engimprove/internal/testutil"
)

type fakeTranscriber struct {
	text string
	cost float64
}

func (f *fakeTranscriber) Transcribe(ctx context.Context, audio []byte, ext string) (string, float64, float64, error) {
	return f.text, 1.5, f.cost, nil
}
func (f *fakeTranscriber) Backend() string { return "fake" }
func (f *fakeTranscriber) Model() string   { return "fake-model" }

func TestSaveRecordsCost(t *testing.T) {
	st, _ := testutil.Store(t)
	tr := &fakeTranscriber{text: "hello world", cost: 0.0012}
	rec, err := Save(context.Background(), st, tr, bytes.NewReader([]byte("audio")), ".webm", "prompt", "")
	if err != nil {
		t.Fatal(err)
	}
	if rec.WhisperBackend != "fake" {
		t.Fatalf("backend = %q", rec.WhisperBackend)
	}
	if rec.WhisperCost != 0.0012 {
		t.Fatalf("cost = %v", rec.WhisperCost)
	}
	if rec.Transcript != "hello world" {
		t.Fatalf("transcript = %q", rec.Transcript)
	}
}

func TestApplyInterview(t *testing.T) {
	st, _ := testutil.Store(t)
	tr := &fakeTranscriber{text: "I did a thing"}
	rec, err := Save(context.Background(), st, tr, bytes.NewReader([]byte("audio")), ".webm", "Tell me about a time", "")
	if err != nil {
		t.Fatal(err)
	}
	review := coach.InterviewReview{OverallScore: 4, Rewrite: "I fixed the bug."}
	rec, err = ApplyInterview(st, rec, review, "openrouter test")
	if err != nil {
		t.Fatal(err)
	}
	if rec.InterviewReview == nil || rec.InterviewReview.OverallScore != 4 {
		t.Fatal("interview review not stored")
	}
	if rec.ReviewBackend != "openrouter test" {
		t.Fatalf("review backend = %q", rec.ReviewBackend)
	}
}

func TestTotalSpend(t *testing.T) {
	st, _ := testutil.Store(t)
	for _, tc := range []struct {
		text string
		wc   float64
		rc   float64
	}{
		{"one", 0.001, 0.002},
		{"two", 0.003, 0.000},
	} {
		tr := &fakeTranscriber{text: tc.text, cost: tc.wc}
		rec, err := Save(context.Background(), st, tr, bytes.NewReader([]byte("audio")), ".webm", "p", "")
		if err != nil {
			t.Fatal(err)
		}
		rec.WhisperCost = tc.wc
		rec.ReviewCost = tc.rc
		if err := write(st, rec); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Second)
	}
	total, err := TotalSpend(st)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0.006 {
		t.Fatalf("total = %v", total)
	}
}

func TestPendingSkipsInterviewReviewed(t *testing.T) {
	st, _ := testutil.Store(t)
	tr := &fakeTranscriber{text: "x"}
	rec, err := Save(context.Background(), st, tr, bytes.NewReader([]byte("audio")), ".webm", "p", "")
	if err != nil {
		t.Fatal(err)
	}
	rec, err = ApplyInterview(st, rec, coach.InterviewReview{OverallScore: 3}, "backend")
	if err != nil {
		t.Fatal(err)
	}
	pending, err := Pending(st)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending, got %d", len(pending))
	}
}

func TestLocalTranscriberMissingTools(t *testing.T) {
	tr := NewLocalTranscriber(Tools{Ready: false, Missing: "ffmpeg"})
	_, _, _, err := tr.Transcribe(context.Background(), []byte("x"), ".webm")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWavSeconds(t *testing.T) {
	f := filepath.Join(t.TempDir(), "audio.wav")
	// 44-byte header + 32000 bytes = 1 second of 16kHz mono 16-bit.
	data := make([]byte, 32044)
	if err := os.WriteFile(f, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := wavSeconds(f); got != 1 {
		t.Fatalf("wavSeconds = %v", got)
	}
}
