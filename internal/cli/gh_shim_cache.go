package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/openclaw/gitcrawl/internal/config"
)

func (a *App) execRealGHMaybeCached(ctx context.Context, args []string) error {
	if !cacheableGHRead(args) {
		err := a.execRealGH(ctx, args)
		if err == nil && mutatingGHCommand(args) {
			_ = a.incrementGHXCacheCounter("pass_through_writes")
			_ = a.clearGHCommandCache()
		}
		return err
	}
	cacheDir, err := a.ghCommandCacheDir()
	if err != nil {
		return a.execRealGH(ctx, args)
	}
	ttl := a.ghCommandCacheTTL(ctx, args)
	entryPath := filepath.Join(cacheDir, a.ghCommandCacheKey(ctx, args)+".json")
	if entry, ok := readGHCommandCache(entryPath, ttl); ok {
		_ = a.incrementGHXCacheCounter("fallback_hits")
		return a.writeGHCommandCacheEntry(entry)
	}
	lockPath := entryPath + ".lock"
	lock, locked := tryGHCommandCacheLock(lockPath)
	if !locked {
		if entry, ok := waitGHCommandCache(entryPath, lockPath, ttl); ok {
			_ = a.incrementGHXCacheCounter("fallback_hits")
			return a.writeGHCommandCacheEntry(entry)
		}
		lock, locked = tryGHCommandCacheLock(lockPath)
	}
	if locked {
		defer func() {
			_ = lock.Close()
			_ = os.Remove(lockPath)
		}()
		if entry, ok := readGHCommandCache(entryPath, ttl); ok {
			_ = a.incrementGHXCacheCounter("fallback_hits")
			return a.writeGHCommandCacheEntry(entry)
		}
	}

	stdout, stderr, exitCode, err := a.captureRealGH(ctx, args)
	_ = a.incrementGHXCacheCounter("backend_misses")
	if err == nil {
		_ = writeGHCommandCache(entryPath, ghCommandCacheEntry{
			CreatedAt: time.Now().UTC(),
			Args:      append([]string(nil), args...),
			ExitCode:  exitCode,
			Stdout:    stdout,
			Stderr:    stderr,
		})
	}
	_, _ = io.WriteString(a.Stdout, stdout)
	_, _ = io.WriteString(a.Stderr, stderr)
	return err
}

func (a *App) captureRealGH(ctx context.Context, args []string) (string, string, int, error) {
	ghPath := strings.TrimSpace(os.Getenv("GITCRAWL_GH_PATH"))
	if ghPath == "" {
		if _, err := os.Stat("/opt/homebrew/opt/gh/bin/gh"); err == nil {
			ghPath = "/opt/homebrew/opt/gh/bin/gh"
		} else {
			var err error
			ghPath, err = exec.LookPath("gh")
			if err != nil {
				return "", "", 127, fmt.Errorf("real gh not found; set GITCRAWL_GH_PATH")
			}
		}
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, ghPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitCode = 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}
	return stdout.String(), stderr.String(), exitCode, err
}

func (a *App) ghCommandCacheDir() (string, error) {
	cfg, err := config.Load(a.configPath)
	if err != nil {
		cfg = config.Default()
	}
	dir := filepath.Join(cfg.CacheDir, "gh-shim")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func (a *App) clearGHCommandCache() error {
	_, err := a.clearGHCommandCacheCount()
	return err
}

func (a *App) clearGHCommandCacheCount() (int, error) {
	dir, err := a.ghCommandCacheDir()
	if err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".lock") || isGHCommandCacheEntryFile(name) {
			if err := os.Remove(filepath.Join(dir, entry.Name())); err == nil {
				removed++
			}
		}
	}
	return removed, nil
}

const ghXCacheStatsFile = "_stats.json"

func isGHCommandCacheEntryFile(name string) bool {
	return strings.HasSuffix(name, ".json") && !strings.HasPrefix(name, "_")
}

type ghCommandCacheEntry struct {
	CreatedAt time.Time `json:"created_at"`
	Args      []string  `json:"args"`
	ExitCode  int       `json:"exit_code"`
	Stdout    string    `json:"stdout"`
	Stderr    string    `json:"stderr"`
}

func (a *App) writeGHCommandCacheEntry(entry ghCommandCacheEntry) error {
	_, _ = io.WriteString(a.Stdout, entry.Stdout)
	_, _ = io.WriteString(a.Stderr, entry.Stderr)
	if entry.ExitCode != 0 {
		return fmt.Errorf("cached gh command failed with exit code %d", entry.ExitCode)
	}
	return nil
}

func readGHCommandCache(path string, ttl time.Duration) (ghCommandCacheEntry, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ghCommandCacheEntry{}, false
	}
	var entry ghCommandCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return ghCommandCacheEntry{}, false
	}
	if entry.CreatedAt.IsZero() || time.Since(entry.CreatedAt) > ttl {
		return ghCommandCacheEntry{}, false
	}
	return entry, true
}

func writeGHCommandCache(path string, entry ghCommandCacheEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func tryGHCommandCacheLock(path string) (*os.File, bool) {
	lock, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, false
	}
	_, _ = fmt.Fprintf(lock, "%d\n", os.Getpid())
	return lock, true
}

func waitGHCommandCache(entryPath, lockPath string, ttl time.Duration) (ghCommandCacheEntry, bool) {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		if entry, ok := readGHCommandCache(entryPath, ttl); ok {
			return entry, true
		}
		if _, err := os.Stat(lockPath); os.IsNotExist(err) {
			return ghCommandCacheEntry{}, false
		}
	}
	_ = os.Remove(lockPath)
	return ghCommandCacheEntry{}, false
}

func (a *App) ghCommandCacheKey(ctx context.Context, args []string) string {
	cwd, _ := os.Getwd()
	material := strings.Join([]string{
		"v2",
		config.ResolvePath(a.configPath),
		cwd,
		os.Getenv("GH_HOST"),
		os.Getenv("GH_REPO"),
		a.ghCommandStableIdentity(ctx, args),
		strings.Join(args, "\x00"),
	}, "\x00")
	sum := sha256.Sum256([]byte(material))
	return hex.EncodeToString(sum[:])
}

func (a *App) ghCommandStableIdentity(ctx context.Context, args []string) string {
	if !isGHPRDiff(args) {
		return ""
	}
	repo, number, ok := parseGHPRDiffIdentityArgs(args)
	if !ok {
		return ""
	}
	thread, err := a.localGHThread(ctx, repo, "pull_request", number)
	if err != nil {
		return ""
	}
	sha := ghPRHeadSHAFromRawJSON(thread.RawJSON)
	if sha == "" {
		return ""
	}
	return fmt.Sprintf("pr-diff:%s:%d:%s", repo, number, sha)
}

func cacheableGHRead(args []string) bool {
	if len(args) == 0 || hasAnyGHFlag(args, "--web", "--browser", "--interactive") {
		return false
	}
	switch args[0] {
	case "api":
		return ghAPIReadOnly(args[1:])
	case "run":
		return len(args) >= 2 && (args[1] == "list" || args[1] == "view")
	case "pr":
		return len(args) >= 2 && (args[1] == "diff" || args[1] == "checks" || args[1] == "view")
	case "issue":
		return len(args) >= 2 && args[1] == "view"
	case "repo":
		return len(args) >= 2 && (args[1] == "view" || args[1] == "list")
	case "label":
		return len(args) >= 2 && args[1] == "list"
	default:
		return false
	}
}

func ghCommandName(args []string) string {
	if len(args) == 0 {
		return ""
	}
	if len(args) == 1 {
		return args[0]
	}
	return args[0] + " " + args[1]
}

func ghAPIReadOnly(args []string) bool {
	method := "GET"
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--input", "-F", "-f", "--field", "--raw-field":
			return false
		case "--method", "-X":
			if index+1 >= len(args) {
				return false
			}
			method = strings.ToUpper(args[index+1])
			index++
		default:
			if strings.HasPrefix(arg, "--method=") {
				method = strings.ToUpper(strings.TrimPrefix(arg, "--method="))
			}
		}
	}
	return method == "GET"
}

func (a *App) ghCommandCacheTTL(ctx context.Context, args []string) time.Duration {
	return ghCommandCacheTTLBase(args, a.ghCommandStableIdentity(ctx, args) != "")
}

func ghCommandCacheTTL(args []string) time.Duration {
	return ghCommandCacheTTLBase(args, false)
}

func ghCommandCacheTTLBase(args []string, stablePRDiff bool) time.Duration {
	if raw := strings.TrimSpace(os.Getenv("GITCRAWL_GH_CACHE_TTL")); raw != "" {
		if duration, err := time.ParseDuration(raw); err == nil && duration > 0 {
			return duration
		}
	}
	if len(args) >= 2 {
		if args[0] == "pr" && args[1] == "diff" {
			if stablePRDiff {
				return 7 * 24 * time.Hour
			}
			return 5 * time.Minute
		}
		if args[0] == "api" {
			return time.Minute
		}
	}
	return 30 * time.Second
}

func isGHPRDiff(args []string) bool {
	return len(args) >= 2 && args[0] == "pr" && args[1] == "diff"
}

func parseGHPRDiffIdentityArgs(args []string) (string, int, bool) {
	if !isGHPRDiff(args) {
		return "", 0, false
	}
	var repo string
	var number int
	for index := 2; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "-R", "--repo":
			if index+1 >= len(args) {
				return "", 0, false
			}
			repo = strings.TrimSpace(args[index+1])
			index++
		default:
			if strings.HasPrefix(arg, "--repo=") {
				repo = strings.TrimSpace(strings.TrimPrefix(arg, "--repo="))
				continue
			}
			if strings.HasPrefix(arg, "-") || number != 0 {
				continue
			}
			parsed, err := parseThreadNumber(arg)
			if err != nil {
				return "", 0, false
			}
			number = parsed
		}
	}
	if repo == "" {
		if envRepo := strings.TrimSpace(os.Getenv("GH_REPO")); envRepo != "" {
			repo = envRepo
		}
	}
	return repo, number, repo != "" && number > 0
}

func ghPRHeadSHAFromRawJSON(raw string) string {
	var payload struct {
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Head.SHA)
}

func mutatingGHCommand(args []string) bool {
	if len(args) < 2 {
		return false
	}
	switch args[0] {
	case "issue":
		switch args[1] {
		case "close", "comment", "create", "delete", "edit", "lock", "pin", "reopen", "transfer", "unlock", "unpin":
			return true
		}
	case "pr":
		switch args[1] {
		case "checkout":
			return false
		case "close", "comment", "create", "edit", "lock", "merge", "ready", "reopen", "review", "unlock":
			return true
		}
	case "api":
		return !ghAPIReadOnly(args[1:])
	}
	return false
}

func hasAnyGHFlag(args []string, flags ...string) bool {
	for _, arg := range args {
		for _, flag := range flags {
			if arg == flag || strings.HasPrefix(arg, flag+"=") {
				return true
			}
		}
	}
	return false
}
