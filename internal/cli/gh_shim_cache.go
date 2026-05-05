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
	if err == nil || cacheGHReadErrors() {
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

func cacheGHReadErrors() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv("GITCRAWL_GH_CACHE_ERRORS")), "0")
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
	sha := ""
	owner, repoName, err := parseOwnerRepo(repo)
	if err == nil {
		if rt, openErr := a.openLocalRuntimeReadOnly(ctx); openErr == nil {
			if localRepo, repoErr := rt.repository(ctx, owner, repoName); repoErr == nil {
				if cache, cacheErr := rt.Store.PullRequestCache(ctx, localRepo.ID, number); cacheErr == nil {
					sha = cache.Detail.HeadSHA
				}
			}
			_ = rt.Store.Close()
		}
	}
	if sha == "" {
		sha = ghPRHeadSHAFromRawJSON(thread.RawJSON)
	}
	if sha == "" {
		return ""
	}
	return fmt.Sprintf("pr-diff:%s:%d:%s", repo, number, sha)
}
