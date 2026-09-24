package prime

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	runTimeout     = 90 * time.Second
	offlineWaitMax = 30 * time.Minute
	gapBetween     = 5 * time.Second
)

// OnlineChecker reports whether upstream APIs are reachable.
type OnlineChecker interface {
	Online() bool
	WaitOnline(context.Context) bool
}

// RunResult is one primer execution outcome.
type RunResult struct {
	Agent    string
	Slot     string
	OK       bool
	Seconds  float64
	ExitCode int
	Note     string
}

// Command returns the exact shell command for one agent (for dry-run).
func Command(agent string, cfg Settings) string {
	argv := argv(agent, cfg)
	if len(argv) == 0 {
		return ""
	}
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = shellQuote(a)
	}
	return strings.Join(parts, " ")
}

// RunItem executes one due primer with offline wait and timeout.
func RunItem(ctx context.Context, item Item, cfg Settings, slotTime time.Time, net OnlineChecker) RunResult {
	res := RunResult{Agent: item.Agent, Slot: item.Slot, ExitCode: -1}
	if net != nil && !net.Online() {
		deadline := slotTime.Add(offlineWaitMax)
		waitCtx, cancel := context.WithDeadline(ctx, deadline)
		log.Printf("[prime] %s waiting for network until %s", item.Agent, deadline.Format("15:04"))
		if !net.WaitOnline(waitCtx) {
			cancel()
			res.Note = "offline, gave up for today"
			log.Printf("[prime] %s offline past catch-up window", item.Agent)
			appendCSV(res)
			return res
		}
		cancel()
	}
	start := time.Now()
	exitCode, firstLine, err := execAgent(ctx, item.Agent, cfg)
	res.Seconds = time.Since(start).Seconds()
	res.ExitCode = exitCode
	if err != nil {
		res.Note = firstLine
		if res.Note == "" {
			res.Note = err.Error()
		}
		log.Printf("[prime] %s failed: %s", item.Agent, res.Note)
	} else if exitCode == 0 {
		res.OK = true
		log.Printf("[prime] %s ok in %.1fs", item.Agent, res.Seconds)
	} else {
		res.Note = firstLine
		if res.Note == "" {
			res.Note = fmt.Sprintf("exit %d", exitCode)
		}
		log.Printf("[prime] %s failed: %s", item.Agent, res.Note)
	}
	appendCSV(res)
	return res
}

// RunNow executes all configured agents immediately (ignores schedule).
func RunNow(ctx context.Context, cfg Settings, net OnlineChecker) []RunResult {
	if len(cfg.Agents) == 0 {
		return nil
	}
	var out []RunResult
	for i, agent := range cfg.Agents {
		if i > 0 {
			select {
			case <-ctx.Done():
				return out
			case <-time.After(gapBetween):
			}
		}
		item := Item{Agent: agent, Slot: "now"}
		res := RunItem(ctx, item, cfg, time.Now(), net)
		out = append(out, res)
	}
	return out
}

// RunDue executes due items sequentially with a gap between agents.
func RunDue(ctx context.Context, now time.Time, cfg Settings, lastRun map[string]time.Time, net OnlineChecker) ([]RunResult, map[string]PrimeLast) {
	items := Items(now, cfg, lastRun)
	if len(items) == 0 {
		return nil, nil
	}
	today := dateOnly(now)
	updated := map[string]PrimeLast{}
	var out []RunResult
	for i, item := range items {
		if i > 0 {
			select {
			case <-ctx.Done():
				return out, updated
			case <-time.After(gapBetween):
			}
		}
		slotTime, err := slotOnDay(today, item.Slot)
		if err != nil {
			slotTime = now
		}
		res := RunItem(ctx, item, cfg, slotTime, net)
		out = append(out, res)
		updated[item.Agent] = PrimeLast{
			TS:   time.Now().Unix(),
			OK:   res.OK,
			Slot: item.Slot,
		}
		lastRun[runKey(item.Agent, item.Slot)] = time.Now()
	}
	return out, updated
}

func execAgent(ctx context.Context, agent string, cfg Settings) (exitCode int, firstLine string, err error) {
	argv := argv(agent, cfg)
	if len(argv) == 0 {
		return 1, "unknown agent", fmt.Errorf("unknown agent %q", agent)
	}
	home, _ := os.UserHomeDir()
	shellCmd := strings.Join(shellQuoteArgs(argv), " ")
	runCtx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "/bin/zsh", "-lc", shellCmd)
	cmd.Dir = home
	var combined strings.Builder
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	err = cmd.Run()
	firstLine = firstOutputLine(combined.String())
	if runCtx.Err() == context.DeadlineExceeded {
		return 124, firstLine, fmt.Errorf("timeout after %s", runTimeout)
	}
	if err == nil {
		return 0, firstLine, nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), firstLine, err
	}
	return 1, firstLine, err
}

func argv(agent string, cfg Settings) []string {
	msg := cfg.Message
	if msg == "" {
		msg = "hi"
	}
	switch agent {
	case "claude":
		model := cfg.Model
		if model == "" {
			model = "claude-haiku-4-5"
		}
		return []string{"claude", "-p", msg, "--model", model, "--output-format", "text"}
	case "codex":
		out := []string{"codex", "exec", "--skip-git-repo-check", msg}
		if cfg.ModelCodex != "" {
			out = append(out, "-m", cfg.ModelCodex)
		}
		return out
	default:
		return nil
	}
}

func shellQuoteArgs(argv []string) []string {
	out := make([]string, len(argv))
	for i, a := range argv {
		out[i] = shellQuote(a)
	}
	return out
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func firstOutputLine(s string) string {
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			return line
		}
	}
	return strings.TrimSpace(s)
}

func csvPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Logs", "keepgoing", "prime.csv")
}

func appendCSV(res RunResult) {
	dir := filepath.Dir(csvPath())
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.OpenFile(csvPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("[prime] csv: %v", err)
		return
	}
	defer f.Close()
	info, _ := f.Stat()
	w := csv.NewWriter(f)
	if info != nil && info.Size() == 0 {
		_ = w.Write([]string{"ts_iso", "agent", "ok", "seconds", "exit_code", "note"})
	}
	ok := "0"
	if res.OK {
		ok = "1"
	}
	_ = w.Write([]string{
		time.Now().Format(time.RFC3339),
		res.Agent,
		ok,
		fmt.Sprintf("%.1f", res.Seconds),
		fmt.Sprintf("%d", res.ExitCode),
		res.Note,
	})
	w.Flush()
}

// ReadCSVTail returns the last n rows from prime.csv (excluding header).
func ReadCSVTail(n int) ([][]string, error) {
	b, err := os.ReadFile(csvPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r := csv.NewReader(strings.NewReader(string(b)))
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) <= 1 {
		return nil, nil
	}
	data := rows[1:]
	if len(data) <= n {
		return data, nil
	}
	return data[len(data)-n:], nil
}

// NotifyFailure posts a user notification for a failed scheduled primer.
func NotifyFailure(slot, reason string) {
	slotLabel := slot
	if slotLabel == "" {
		slotLabel = "scheduled"
	}
	msg := fmt.Sprintf("Couldn't start your %s session window: %s", slotLabel, reason)
	script := fmt.Sprintf(`display notification %q with title "KeepGoing"`, msg)
	_ = exec.Command("osascript", "-e", script).Run()
}
