// Package speak records spoken answers, transcribes them with whisper.cpp and keeps their reviews.
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
	ID         string  `json:"id"`
	TS         string  `json:"ts"`
	Prompt     string  `json:"prompt"`
	Focus      string  `json:"focus,omitempty"`
	Transcript string  `json:"transcript"`
	Seconds    float64 `json:"seconds"`
	Words      int     `json:"words"`
	Review     *Review `json:"review,omitempty"`
}

// Review is the feedback on a recording.
type Review struct {
	coach.SpeechReview
	Logged     []logbook.Logged `json:"logged"`
	ReviewedBy string           `json:"reviewed_by"`
	TS         string           `json:"ts"`
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
func Save(ctx context.Context, s data.Store, tools Tools, audio io.Reader, ext, prompt, focus string) (Recording, error) {
	if !tools.Ready {
		return Recording{}, fmt.Errorf("speaking pipeline not ready: missing %s", tools.Missing)
	}
	now := time.Now()
	rec := Recording{ID: now.Format("20060102-150405"), TS: now.Format(time.RFC3339), Prompt: prompt, Focus: focus}
	dir := s.Path("speaking", rec.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return rec, err
	}
	src := filepath.Join(dir, "upload"+ext)
	f, err := os.Create(src)
	if err != nil {
		return rec, err
	}
	if _, err := io.Copy(f, audio); err != nil {
		f.Close()
		return rec, err
	}
	if err := f.Close(); err != nil {
		return rec, err
	}
	wav := filepath.Join(dir, "audio.wav")
	if out, err := run(ctx, tools.FFmpeg, "-y", "-loglevel", "error", "-i", src, "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", wav); err != nil {
		return rec, fmt.Errorf("ffmpeg: %w: %s", err, out)
	}
	rec.Seconds = wavSeconds(wav)
	prefix := filepath.Join(dir, "transcript")
	if out, err := run(ctx, tools.Whisper, "-m", tools.Model, "-l", "en", "-nt", "-np", "-otxt", "-of", prefix, "-f", wav); err != nil {
		return rec, fmt.Errorf("whisper-cli: %w: %s", err, out)
	}
	raw, err := os.ReadFile(prefix + ".txt")
	if err != nil {
		return rec, err
	}
	rec.Transcript = strings.Join(strings.Fields(string(raw)), " ")
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
		if r.Review == nil && r.Transcript != "" {
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
