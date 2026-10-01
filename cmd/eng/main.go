// Command eng is the English learning machine: web app, Claude Code hook and drill cli.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"engimprove/internal/cards"
	"engimprove/internal/coach"
	"engimprove/internal/config"
	"engimprove/internal/data"
	"engimprove/internal/hook"
	"engimprove/internal/logbook"
	"engimprove/internal/server"
	"engimprove/internal/speak"
	"engimprove/internal/topics"
	"engimprove/web"
)

const usage = `eng — English learning machine

  eng serve [-addr 127.0.0.1:7421]        web app + api
  eng open [route]                        start the server if needed and open the browser (route: drill/the, speak, review)
  eng restart                             restart the background server after a rebuild
  eng status [-json]                      ripe drills, due reviews, speaking queue
  eng log [-f file.json]                  append mistakes (logbook json on stdin or -f), print repeat counts
  eng drill next [-topic T] [-n 5]        cards as json (topic "review" = due cards)
  eng drill answer <card-id> ok|fail      grade one card
  eng drill done -topic T -asked N -correct M
  eng speak pending|review                list or review unreviewed recordings (review runs claude -p)
  eng speak apply <id> -f review.json     store a review written by a Claude Code session
  eng pack generate -topic T [-n 12]      new exercises from your mistakes via claude -p
  eng pack add -topic T -f items.json     add exercises written by a Claude Code session
  eng whisper-setup [-model small.en]     download a whisper.cpp model
  eng check <queue-file>                  check one queued prompt (spawned by the hook, detached)
  eng check -retry                        re-run prompts whose check failed
  eng hook                                UserPromptSubmit hook, async: queue English prompts
  eng hook-stop                           Stop hook: show corrections and ripe drills as a system message
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "eng: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Print(usage)
		return nil
	}
	switch args[0] {
	case "hook":
		return runHook()
	case "hook-stop":
		return runStopHook()
	}
	st, err := data.Open()
	if err != nil {
		return err
	}
	cfg, err := config.Load(st)
	if err != nil {
		return err
	}
	switch args[0] {
	case "serve":
		return serve(st, cfg, args[1:])
	case "open":
		return openApp(st, cfg, args[1:])
	case "restart":
		return restart(st, cfg)
	case "status":
		return status(st, cfg, args[1:])
	case "log":
		return logCmd(st, cfg, args[1:])
	case "drill":
		return drill(st, cfg, args[1:])
	case "speak":
		return speakCmd(st, cfg, args[1:])
	case "pack":
		return pack(st, cfg, args[1:])
	case "whisper-setup":
		return whisperSetup(st, cfg, args[1:])
	case "check":
		return check(st, cfg, args[1:])
	case "doctor":
		return doctor(st, cfg)
	}
	return fmt.Errorf("unknown command %q\n\n%s", args[0], usage)
}

func stamp() string { return time.Now().Format("15:04:05") }

func progress(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", stamp(), fmt.Sprintf(format, a...))
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func hookContext() (data.Store, config.Config, hook.Input, bool) {
	if os.Getenv("ENG_HOOK_OFF") == "1" {
		return data.Store{}, config.Config{}, hook.Input{}, false
	}
	var in hook.Input
	if err := json.NewDecoder(os.Stdin).Decode(&in); err != nil {
		fmt.Fprintln(os.Stderr, "eng hook: bad payload:", err)
		return data.Store{}, config.Config{}, in, false
	}
	st, err := data.Open()
	if err != nil {
		fmt.Fprintln(os.Stderr, "eng hook:", err)
		return st, config.Config{}, in, false
	}
	cfg, err := config.Load(st)
	if err != nil {
		fmt.Fprintln(os.Stderr, "eng hook:", err)
		return st, cfg, in, false
	}
	return st, cfg, in, true
}

// runHook queues English prompts and hands them to a detached checker; it never blocks or fails the prompt.
func runHook() error {
	st, _, in, ok := hookContext()
	if !ok {
		return nil
	}
	path, err := hook.Prompt(st, in, time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, "eng hook:", err)
		return nil
	}
	if path == "" {
		return nil
	}
	if err := spawn(st, "check.log", "check", path); err != nil {
		fmt.Fprintln(os.Stderr, "eng hook:", err)
	}
	return nil
}

// runStopHook prints corrections and drill nudges as a system message; it never blocks stopping.
func runStopHook() error {
	st, cfg, in, ok := hookContext()
	if !ok {
		return nil
	}
	msg, notice, err := hook.Stop(st, cfg, in.SessionID, time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, "eng hook-stop:", err)
	}
	if notice != nil {
		notify(notice)
	}
	if msg == "" {
		return nil
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]string{"systemMessage": msg})
}

// spawn starts this binary detached so it outlives the hook and the session.
func spawn(st data.Store, logName string, args ...string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	logPath := st.Path("state", logName)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()
	cmd := exec.Command(self, args...)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func doctor(st data.Store, cfg config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for k, v := range coach.New(cfg).Probe(ctx) {
		fmt.Printf("%-11s %s\n", k, v)
	}
	tools := speak.Detect(st, cfg.WhisperModel)
	fmt.Printf("%-11s ready=%v %s\n", "speaking", tools.Ready, tools.Missing)
	fmt.Printf("%-11s %v\n", "server", alive(cfg))
	failed, _ := filepath.Glob(st.Path("state", "queue", "failed", "*.json"))
	fmt.Printf("%-11s %d failed checks (eng check -retry)\n", "queue", len(failed))
	return nil
}

func check(st data.Store, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	retry := fs.Bool("retry", false, "re-run prompts from state/queue/failed")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var paths []string
	if *retry {
		failed, err := filepath.Glob(st.Path("state", "queue", "failed", "*.json"))
		if err != nil {
			return err
		}
		for _, f := range failed {
			dest := st.Path("state", "queue", filepath.Base(f))
			if err := os.Rename(f, dest); err != nil {
				return err
			}
			paths = append(paths, dest)
		}
	} else {
		paths = fs.Args()
	}
	if len(paths) == 0 {
		return errors.New("usage: eng check <queue-file> | eng check -retry")
	}
	c := coach.New(cfg)
	for _, p := range paths {
		n, backend, err := hook.Check(context.Background(), st, cfg, c, p)
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(p), err)
		}
		progress("%s: %d mistakes via %s", filepath.Base(p), n, backend)
	}
	return nil
}

func notify(n *hook.Notice) {
	bin, err := exec.LookPath("terminal-notifier")
	if err != nil {
		bin = "/opt/homebrew/bin/terminal-notifier"
		if _, err := os.Stat(bin); err != nil {
			return
		}
	}
	self, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(bin, "-title", "engimprove", "-subtitle", "time to practise", "-message", n.Message,
		"-group", "engimprove", "-execute", fmt.Sprintf("%q open %s", self, n.Route))
	_ = cmd.Start()
}

func serve(st data.Store, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", cfg.Addr, "listen address, keep it on loopback")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg.Addr = *addr
	srv := &server.Server{Store: st, Config: cfg, Coach: coach.New(cfg), Static: web.Static(),
		Log: log.New(os.Stderr, "", log.LstdFlags)}
	httpSrv := &http.Server{Addr: *addr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdown)
	}()
	progress("serving %s on http://%s", st.Root, *addr)
	pidPath := st.Path("state", "serve.pid")
	_ = data.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())))
	defer os.Remove(pidPath)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// restart stops the running server so a rebuilt binary serves the new frontend.
func restart(st data.Store, cfg config.Config) error {
	if raw, err := os.ReadFile(st.Path("state", "serve.pid")); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil {
			if p, err := os.FindProcess(pid); err == nil {
				_ = p.Signal(syscall.SIGTERM)
			}
		}
	}
	for range 50 {
		if !alive(cfg) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if alive(cfg) {
		return fmt.Errorf("server on %s did not stop; it was not started by this eng (no state/serve.pid)", cfg.Addr)
	}
	if err := spawn(st, "serve.log", "serve"); err != nil {
		return err
	}
	for range 30 {
		if alive(cfg) {
			progress("server restarted on %s", cfg.URL())
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("server did not come up, see %s", st.Path("state", "serve.log"))
}

func alive(cfg config.Config) bool {
	client := http.Client{Timeout: 400 * time.Millisecond}
	resp, err := client.Get(cfg.URL() + "/api/ping")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func openApp(st data.Store, cfg config.Config, args []string) error {
	route := ""
	if len(args) > 0 {
		route = strings.TrimPrefix(args[0], "/")
	}
	if !alive(cfg) {
		if err := spawn(st, "serve.log", "serve"); err != nil {
			return err
		}
		progress("started server, log %s", st.Path("state", "serve.log"))
		for range 30 {
			if alive(cfg) {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	url := cfg.URL() + "/"
	if route != "" {
		url += "#/" + route
	}
	fmt.Println(url)
	return exec.Command("open", url).Run()
}

func status(st data.Store, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	entries, err := st.Entries()
	if err != nil {
		return err
	}
	sessions, err := topics.Sessions(st)
	if err != nil {
		return err
	}
	all, err := cards.All(st, cfg, entries)
	if err != nil {
		return err
	}
	srs, err := cards.LoadSRS(st)
	if err != nil {
		return err
	}
	pending, err := speak.Pending(st)
	if err != nil {
		return err
	}
	statuses := topics.Statuses(cfg, entries, sessions)
	due := cards.CountDue(all, srs, time.Now())
	if *asJSON {
		return printJSON(map[string]any{"topics": statuses, "due": due, "speaking_pending": len(pending), "url": cfg.URL()})
	}
	for _, s := range statuses {
		mark := " "
		if s.Ready {
			mark = "●"
		}
		filled := min(s.Fresh*10/max(s.Threshold, 1), 10)
		fmt.Printf("%s %-12s %s%s %2d/%-2d  total %d\n", mark, s.ID, strings.Repeat("█", filled),
			strings.Repeat("░", 10-filled), s.Fresh, s.Threshold, s.Total)
	}
	fmt.Printf("\ndue reviews %d · speaking to review %d · %s\n", due, len(pending), cfg.URL())
	return nil
}

func readInput(path string) ([]byte, error) {
	if path == "" || path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

func logCmd(st data.Store, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("log", flag.ContinueOnError)
	file := fs.String("f", "", "logbook json file, stdin when empty")
	asJSON := fs.Bool("json", false, "print json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	raw, err := readInput(*file)
	if err != nil {
		return err
	}
	var in logbook.Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return fmt.Errorf("input is not a logbook object: %w", err)
	}
	if in.Language == "" {
		in.Language = cfg.Language
	}
	logged, textID, err := logbook.Log(st, in)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(map[string]any{"text_id": textID, "logged": logged})
	}
	fmt.Printf("text_id %s\n", textID)
	for _, l := range logged {
		fmt.Printf("%s  ×%-2d %-22s %s\n", l.ID, l.Count, l.Category, l.Rule)
		if len(l.Similar) > 0 {
			fmt.Printf("    new rule; existing in %s: %s\n", l.Category, strings.Join(l.Similar, " | "))
		}
	}
	return nil
}

func drill(st data.Store, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("drill needs next, answer or done")
	}
	switch args[0] {
	case "next":
		fs := flag.NewFlagSet("drill next", flag.ContinueOnError)
		topic := fs.String("topic", "", "topic id or review; empty picks the ripest topic")
		n := fs.Int("n", 5, "how many cards")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		entries, err := st.Entries()
		if err != nil {
			return err
		}
		if *topic == "" {
			sessions, err := topics.Sessions(st)
			if err != nil {
				return err
			}
			*topic = topics.Statuses(cfg, entries, sessions)[0].ID
		}
		if _, ok := cfg.Topic(*topic); !ok && *topic != cards.Review {
			return fmt.Errorf("unknown topic %q", *topic)
		}
		all, err := cards.All(st, cfg, entries)
		if err != nil {
			return err
		}
		srs, err := cards.LoadSRS(st)
		if err != nil {
			return err
		}
		var lessons map[string]any
		_ = data.ReadJSON(st.Path("content", "lessons.json"), &lessons)
		return printJSON(map[string]any{
			"topic": *topic, "lesson": lessons[*topic], "cards": cards.Select(all, srs, *topic, *n, time.Now()),
		})
	case "answer":
		if len(args) < 3 || (args[2] != "ok" && args[2] != "fail") {
			return errors.New("usage: eng drill answer <card-id> ok|fail")
		}
		entries, err := st.Entries()
		if err != nil {
			return err
		}
		all, err := cards.All(st, cfg, entries)
		if err != nil {
			return err
		}
		card, ok := cards.Find(all, args[1])
		if !ok {
			return fmt.Errorf("unknown card %q", args[1])
		}
		state, err := cards.Record(st, card, args[2] == "ok", "", time.Now())
		if err != nil {
			return err
		}
		fmt.Printf("%s box %d, next %s\n", card.ID, state.Box, state.Due)
		return nil
	case "done":
		fs := flag.NewFlagSet("drill done", flag.ContinueOnError)
		topic := fs.String("topic", "", "topic id or review")
		asked := fs.Int("asked", 0, "cards asked")
		correct := fs.Int("correct", 0, "cards answered right")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *topic == "" || *asked <= 0 || *correct < 0 || *correct > *asked {
			return errors.New("need -topic, -asked > 0 and 0 <= -correct <= -asked")
		}
		sess, err := server.FinishSession(st, cfg, *topic, *asked, *correct)
		if err != nil {
			return err
		}
		fmt.Printf("session logged: %s %d/%d, cursor %s\n", sess.Topic, sess.Correct, sess.Asked, sess.Cursor)
		return nil
	}
	return fmt.Errorf("unknown drill command %q", args[0])
}

func speakCmd(st data.Store, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("speak needs pending, review or apply")
	}
	switch args[0] {
	case "pending":
		pending, err := speak.Pending(st)
		if err != nil {
			return err
		}
		return printJSON(pending)
	case "review":
		pending, err := speak.Pending(st)
		if err != nil {
			return err
		}
		if len(pending) == 0 {
			progress("nothing to review")
			return nil
		}
		c := coach.New(cfg)
		for _, rec := range pending {
			progress("reviewing %s (%d words)", rec.ID, rec.Words)
			review, backend, err := c.ReviewSpeech(context.Background(), st, rec.Prompt, rec.Transcript)
			if err != nil {
				return err
			}
			rec, err = speak.Apply(st, rec, review, backend)
			if err != nil {
				return err
			}
			fmt.Printf("%s: %d mistakes logged\n", rec.ID, len(rec.Review.Logged))
		}
		return nil
	case "apply":
		if len(args) < 2 {
			return errors.New("usage: eng speak apply <id> -f review.json")
		}
		fs := flag.NewFlagSet("speak apply", flag.ContinueOnError)
		file := fs.String("f", "", "review json, stdin when empty")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		rec, err := speak.Load(st, args[1])
		if err != nil {
			return err
		}
		raw, err := readInput(*file)
		if err != nil {
			return err
		}
		var review coach.SpeechReview
		if err := json.Unmarshal(raw, &review); err != nil {
			return err
		}
		rec, err = speak.Apply(st, rec, review, "claude code session")
		if err != nil {
			return err
		}
		fmt.Printf("%s: %d mistakes logged\n", rec.ID, len(rec.Review.Logged))
		return nil
	}
	return fmt.Errorf("unknown speak command %q", args[0])
}

func pack(st data.Store, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return errors.New("pack needs generate or add")
	}
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	topic := fs.String("topic", "", "topic id")
	n := fs.Int("n", 12, "how many exercises")
	file := fs.String("f", "", "items json array, stdin when empty")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	t, ok := cfg.Topic(*topic)
	if !ok {
		return fmt.Errorf("unknown topic %q", *topic)
	}
	var items []cards.PackItem
	switch args[0] {
	case "generate":
		entries, err := st.Entries()
		if err != nil {
			return err
		}
		mistakes := server.TopicMistakes(cfg, entries, t.ID, 25)
		if len(mistakes) == 0 {
			return fmt.Errorf("no mistakes in topic %s yet", t.ID)
		}
		c := coach.New(cfg)
		backend, err := c.Backend(false)
		if err != nil {
			return err
		}
		progress("asking %s for %d %s exercises", backend, *n, t.ID)
		items, err = c.GeneratePack(context.Background(), t.Title, mistakes, *n)
		if err != nil {
			return err
		}
	case "add":
		raw, err := readInput(*file)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &items); err != nil {
			return fmt.Errorf("items must be a json array of pack items: %w", err)
		}
	default:
		return fmt.Errorf("unknown pack command %q", args[0])
	}
	added, err := cards.AppendPack(st, t.ID, items)
	if err != nil {
		return err
	}
	fmt.Printf("%d exercises added to drills/packs/%s.jsonl\n", added, t.ID)
	return nil
}

func whisperSetup(st data.Store, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("whisper-setup", flag.ContinueOnError)
	model := fs.String("model", "small.en", "whisper.cpp model name: base.en, small.en, medium.en, large-v3-turbo")
	if err := fs.Parse(args); err != nil {
		return err
	}
	dest := st.Path("models", "ggml-"+*model+".bin")
	if info, err := os.Stat(dest); err == nil && info.Size() > 10<<20 {
		progress("%s already present (%d MB)", dest, info.Size()>>20)
		return nil
	}
	url := "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-" + *model + ".bin"
	progress("downloading %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	progress("saved %s (%d MB)", dest, n>>20)
	if want := "models/ggml-" + *model + ".bin"; cfg.WhisperModel != want && cfg.WhisperModel != dest {
		progress("config/eng.json uses %s; set whisper_model to %q to use this one", cfg.WhisperModel, want)
	}
	return nil
}
