// Package speak records spoken answers, transcribes them and keeps their reviews.
package speak

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"engimprove/internal/coach"
	"engimprove/internal/data"
	"engimprove/internal/lang"
	"engimprove/internal/logbook"
)

// Recording is one spoken answer and, once reviewed, its feedback.
type Recording struct {
	ID              string                   `json:"id"`
	TS              string                   `json:"ts"`
	Prompt          string                   `json:"prompt"`
	Focus           string                   `json:"focus,omitempty"`
	Transcript      string                   `json:"transcript"`
	Seconds         float64                  `json:"seconds"`
	Words           int                      `json:"words"`
	WhisperModel    string                   `json:"whisper_model"`
	WhisperBackend  string                   `json:"whisper_backend"`
	WhisperCost     float64                  `json:"whisper_cost"`
	ReviewCost      float64                  `json:"review_cost"`
	ReviewBackend   string                   `json:"review_backend"`
	Review          *Review                  `json:"review,omitempty"`
	InterviewReview *coach.InterviewReview   `json:"interview_review,omitempty"`
}

// Review is the feedback on a recording.
type Review struct {
	coach.SpeechReview
	Logged     []logbook.Logged `json:"logged"`
	ReviewedBy string           `json:"reviewed_by"`
	TS         string           `json:"ts"`
}

// Transcriber turns audio bytes into a transcript.
type Transcriber interface {
	Transcribe(ctx context.Context, audio []byte, ext string) (transcript string, seconds, cost float64, err error)
	Backend() string
	Model() string
}

// LocalTranscriber uses ffmpeg + whisper-cli.
type LocalTranscriber struct {
	Tools Tools
}

// NewLocalTranscriber builds a transcriber from local tools.
func NewLocalTranscriber(t Tools) *LocalTranscriber { return &LocalTranscriber{Tools: t} }

func (l *LocalTranscriber) Backend() string { return "local" }
func (l *LocalTranscriber) Model() string   { return filepath.Base(l.Tools.Model) }

func (l *LocalTranscriber) Transcribe(ctx context.Context, audio []byte, ext string) (string, float64, float64, error) {
	if !l.Tools.Ready {
		return "", 0, 0, fmt.Errorf("speaking pipeline not ready: missing %s", l.Tools.Missing)
	}
	dir, err := os.MkdirTemp("", "eng-speak-*")
	if err != nil {
		return "", 0, 0, err
	}
	defer os.RemoveAll(dir)
	src := filepath.Join(dir, "upload"+ext)
	if err := os.WriteFile(src, audio, 0o644); err != nil {
		return "", 0, 0, err
	}
	wav := filepath.Join(dir, "audio.wav")
	if out, err := run(ctx, l.Tools.FFmpeg, "-y", "-loglevel", "error", "-i", src, "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", wav); err != nil {
		return "", 0, 0, fmt.Errorf("ffmpeg: %w: %s", err, out)
	}
	prefix := filepath.Join(dir, "transcript")
	if out, err := run(ctx, l.Tools.Whisper, "-m", l.Tools.Model, "-l", "en", "-nt", "-np", "-otxt", "-of", prefix, "-f", wav); err != nil {
		return "", 0, 0, fmt.Errorf("whisper-cli: %w: %s", err, out)
	}
	raw, err := os.ReadFile(prefix + ".txt")
	if err != nil {
		return "", 0, 0, err
	}
	return strings.Join(strings.Fields(string(raw)), " "), wavSeconds(wav), 0, nil
}

// Tools reports what the speaking pipeline can use right now.
type Tools struct {
	FFmpeg  string `json:"ffmpeg"`
	Whisper string `json:"whisper"`
	Model   string `json:"model"`
	Ready   bool   `json:"ready"`
	Missing string `json:"missing,omitempty"`
}

// Detect finds ffmpeg, whisper-cli and the model file.
func Detect(s data.Store, model string) Tools {
	t := Tools{}
	t.FFmpeg, _ = exec.LookPath("ffmpeg")
	t.Whisper, _ = exec.LookPath("whisper-cli")
	for _, dir := range []string{"/opt/homebrew/bin", "/usr/local/bin"} {
		if t.FFmpeg == "" {
			if _, err := os.Stat(filepath.Join(dir, "ffmpeg")); err == nil {
				t.FFmpeg = filepath.Join(dir, "ffmpeg")
			}
		}
		if t.Whisper == "" {
			if _, err := os.Stat(filepath.Join(dir, "whisper-cli")); err == nil {
				t.Whisper = filepath.Join(dir, "whisper-cli")
			}
		}
	}
	t.Model = model
	if !filepath.IsAbs(model) {
		t.Model = s.Path(model)
	}
	var missing []string
	if t.FFmpeg == "" {
		missing = append(missing, "ffmpeg (brew install ffmpeg)")
	}
	if t.Whisper == "" {
		missing = append(missing, "whisper-cli (brew install whisper-cpp)")
	}
	if _, err := os.Stat(t.Model); err != nil {
		missing = append(missing, "the Whisper model (eng whisper-setup)")
	}
	t.Ready = len(missing) == 0
	t.Missing = strings.Join(missing, ", ")
	return t
}

// Save stores the uploaded audio, converts it to 16 kHz wav and transcribes it.
func Save(ctx context.Context, s data.Store, tr Transcriber, audio io.Reader, ext, prompt, focus string) (Recording, error) {
	now := time.Now()
	rec := Recording{
		ID:             now.Format("20060102-150405"),
		TS:             now.Format(time.RFC3339),
		Prompt:         prompt,
		Focus:          focus,
		WhisperBackend: tr.Backend(),
		WhisperModel:   tr.Model(),
	}
	dir := s.Path("speaking", rec.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return rec, err
	}
	src := filepath.Join(dir, "upload"+ext)
	dataBytes, err := io.ReadAll(io.LimitReader(audio, 64<<20))
	if err != nil {
		return rec, err
	}
	if err := os.WriteFile(src, dataBytes, 0o644); err != nil {
		return rec, err
	}
	transcript, seconds, cost, err := tr.Transcribe(ctx, dataBytes, ext)
	if err != nil {
		return rec, err
	}
	rec.Transcript = transcript
	rec.Seconds = seconds
	rec.WhisperCost = cost
	rec.Words = lang.Analyze(rec.Transcript).Words
	return rec, write(s, rec)
}

func run(ctx context.Context, bin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return strings.TrimSpace(out.String()), err
}

func wavSeconds(path string) float64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	// 16 kHz mono 16-bit pcm: 32000 bytes per second after a 44-byte header.
	return float64(max(info.Size()-44, 0)) / 32000
}

func write(s data.Store, rec Recording) error {
	return data.WriteJSON(s.Path("speaking", rec.ID, "meta.json"), rec)
}

// Load reads one recording.
func Load(s data.Store, id string) (Recording, error) {
	if strings.ContainsAny(id, "/\\.") || id == "" {
		return Recording{}, errors.New("bad recording id")
	}
	var rec Recording
	path := s.Path("speaking", id, "meta.json")
	if _, err := os.Stat(path); err != nil {
		return rec, fmt.Errorf("recording %s not found", id)
	}
	err := data.ReadJSON(path, &rec)
	return rec, err
}

// List returns recordings, newest first.
func List(s data.Store) ([]Recording, error) {
	dirs, err := filepath.Glob(s.Path("speaking", "*", "meta.json"))
	if err != nil {
		return nil, err
	}
	out := make([]Recording, 0, len(dirs))
	for _, d := range dirs {
		var rec Recording
		if err := data.ReadJSON(d, &rec); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

// Pending lists transcribed recordings that nobody has reviewed yet.
func Pending(s data.Store) ([]Recording, error) {
	all, err := List(s)
	if err != nil {
		return nil, err
	}
	var out []Recording
	for _, r := range all {
		if r.Review == nil && r.InterviewReview == nil && r.Transcript != "" {
			out = append(out, r)
		}
	}
	return out, nil
}

// Apply logs the review's mistakes and stores the feedback with the recording.
func Apply(s data.Store, rec Recording, review coach.SpeechReview, by string) (Recording, error) {
	r := &Review{SpeechReview: review, ReviewedBy: by, TS: time.Now().Format(time.RFC3339)}
	if len(review.Errors) > 0 {
		logged, _, err := logbook.Log(s, logbook.Input{
			TextID:    rec.TS[:10] + "-speaking-" + rec.ID[9:13],
			Date:      rec.TS[:10],
			Source:    "speaking",
			Project:   "engimprove",
			Original:  "(" + rec.Prompt + ") " + rec.Transcript,
			Corrected: review.Corrected,
			Errors:    review.Errors,
		})
		if err != nil {
			return rec, err
		}
		r.Logged = logged
	}
	rec.Review = r
	rec.ReviewBackend = by
	rec.ReviewCost = review.Cost
	return rec, write(s, rec)
}

// ApplyInterview stores an interview/STAR review with the recording.
func ApplyInterview(s data.Store, rec Recording, review coach.InterviewReview, by string) (Recording, error) {
	rec.InterviewReview = &review
	rec.ReviewBackend = by
	rec.ReviewCost = review.Cost
	return rec, write(s, rec)
}

// Question is a speaking prompt with an optional focus on a weak rule.
type Question struct {
	Prompt string `json:"prompt"`
	Focus  string `json:"focus,omitempty"`
}

// Pick returns a random prompt from content/speaking.json, aimed at one of the given weak rules.
func Pick(s data.Store, weakRules []string) (Question, error) {
	var prompts []string
	if err := data.ReadJSON(s.Path("content", "speaking.json"), &prompts); err != nil {
		return Question{}, err
	}
	if len(prompts) == 0 {
		return Question{}, errors.New("content/speaking.json is empty")
	}
	q := Question{Prompt: prompts[rand.IntN(len(prompts))]}
	if len(weakRules) > 0 {
		q.Focus = weakRules[rand.IntN(min(len(weakRules), 3))]
	}
	return q, nil
}

// TotalSpend returns the sum of whisper + review costs across all recordings.
func TotalSpend(s data.Store) (float64, error) {
	all, err := List(s)
	if err != nil {
		return 0, err
	}
	var total float64
	for _, r := range all {
		total += r.WhisperCost + r.ReviewCost
	}
	return total, nil
}
