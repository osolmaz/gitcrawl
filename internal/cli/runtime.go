package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/openclaw/gitcrawl/internal/config"
	"github.com/openclaw/gitcrawl/internal/store"
)

type localRuntime struct {
	Config       config.Config
	Store        *store.Store
	SourceDBPath string
	RemoteSource bool
}

const portableStoreRefreshTimeout = 15 * time.Second
const portableStoreRepairTimeout = 90 * time.Second
const portableStoreRefreshTTL = 2 * time.Minute
const portableStoreRefreshFailureBackoff = time.Minute
const portableStoreMarkerFile = "gitcrawl-portable-store"

var errPortableStoreDirty = errors.New("portable store checkout has local changes")

func (a *App) openLocalRuntime(ctx context.Context) (localRuntime, error) {
	cfg, err := config.LoadRuntime(a.configPath)
	if err != nil {
		return localRuntime{}, err
	}
	sourceDBPath := cfg.DBPath
	remoteSource := false
	if _, ok := portableStoreRoot(cfg.DBPath); ok {
		mirrorPath, _, err := a.ensurePortableRuntimeDB(ctx, cfg.DBPath, false)
		if err != nil {
			return localRuntime{}, err
		}
		cfg.DBPath = mirrorPath
		remoteSource = true
	}
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return localRuntime{}, err
	}
	return localRuntime{Config: cfg, Store: st, SourceDBPath: sourceDBPath, RemoteSource: remoteSource}, nil
}

func (a *App) openLocalRuntimeReadOnly(ctx context.Context) (localRuntime, error) {
	cfg, err := config.LoadRuntime(a.configPath)
	if err != nil {
		return localRuntime{}, err
	}
	sourceDBPath := cfg.DBPath
	remoteSource := false
	if _, ok := portableStoreRoot(cfg.DBPath); ok {
		mirrorPath, _, err := a.ensurePortableRuntimeDB(ctx, cfg.DBPath, true)
		if err != nil {
			return localRuntime{}, err
		}
		cfg.DBPath = mirrorPath
		remoteSource = true
	}
	st, err := store.OpenReadOnly(ctx, cfg.DBPath)
	if err != nil {
		return localRuntime{}, err
	}
	return localRuntime{Config: cfg, Store: st, SourceDBPath: sourceDBPath, RemoteSource: remoteSource}, nil
}

func (rt localRuntime) repository(ctx context.Context, owner, repo string) (store.Repository, error) {
	return rt.Store.RepositoryByFullName(ctx, owner+"/"+repo)
}

func (rt localRuntime) defaultRepository(ctx context.Context) (store.Repository, error) {
	repos, err := rt.Store.ListRepositories(ctx)
	if err != nil {
		return store.Repository{}, err
	}
	if len(repos) == 0 {
		return store.Repository{}, fmt.Errorf("no local repositories found")
	}
	return repos[0], nil
}

func refreshPortableStoreForDB(ctx context.Context, dbPath string) error {
	root, ok := portableStoreRoot(dbPath)
	if !ok {
		return nil
	}
	if !portableStoreIsGitWorktree(ctx, root) {
		return nil
	}
	if !gitWorktreeClean(ctx, root) {
		return errPortableStoreDirty
	}
	pullCtx, cancel := context.WithTimeout(ctx, portableStoreRefreshTimeout)
	defer cancel()
	if err := fastForwardGitCheckout(pullCtx, root, true); err != nil {
		return err
	}
	return removePortableSQLiteSidecars(root)
}

func repairMalformedPortableStoreForDB(ctx context.Context, dbPath, configPath string) error {
	root, ok := portableStoreRoot(dbPath)
	if !ok {
		return nil
	}
	if !portableStoreIsGitWorktree(ctx, root) {
		return nil
	}
	if !portableStoreRepairAllowed(root, configPath) {
		return fmt.Errorf("refuse destructive repair for unmarked portable store checkout %s", root)
	}
	if err := preserveMalformedPortableDB(root, dbPath); err != nil {
		return err
	}
	pullCtx, cancel := context.WithTimeout(ctx, portableStoreRepairTimeout)
	defer cancel()
	if !gitWorktreeClean(pullCtx, root) {
		if err := runGit(pullCtx, "", "-C", root, "reset", "--hard", "HEAD"); err != nil {
			return err
		}
	}
	if err := fastForwardGitCheckout(pullCtx, root, true); err != nil {
		return err
	}
	return removePortableSQLiteSidecars(root)
}

var portableRuntimeMu sync.Mutex

func (a *App) ensurePortableRuntimeDB(ctx context.Context, sourceDBPath string, refresh bool) (string, bool, error) {
	mirrorPath, err := a.portableRuntimeDBPath(sourceDBPath)
	if err != nil {
		return "", false, err
	}
	changed, err := refreshPortableRuntimeDB(ctx, sourceDBPath, mirrorPath, refresh, a.configPath)
	return mirrorPath, changed, err
}

func (a *App) portableRuntimeDBPath(sourceDBPath string) (string, error) {
	root, ok := portableStoreRoot(sourceDBPath)
	if !ok {
		return "", fmt.Errorf("portable store root not found for %s", sourceDBPath)
	}
	rel, err := filepath.Rel(root, sourceDBPath)
	if err != nil || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", fmt.Errorf("portable database %s is outside store root %s", sourceDBPath, root)
	}
	name := safePathName(filepath.Base(root))
	if name == "" {
		name = "portable-store"
	}
	return filepath.Join(filepath.Dir(config.ResolvePath(a.configPath)), "runtime", name, rel), nil
}

func refreshPortableRuntimeDB(ctx context.Context, sourceDBPath, mirrorPath string, refresh bool, configPath string) (bool, error) {
	portableRuntimeMu.Lock()
	defer portableRuntimeMu.Unlock()
	portableRoot, isPortableSource := portableStoreRoot(sourceDBPath)
	isRepairablePortableSource := isPortableSource && portableStoreIsGitWorktree(ctx, portableRoot)
	if refresh {
		_ = refreshPortableStoreForDBIfDue(ctx, sourceDBPath, mirrorPath)
	}
	needsCopy, err := portableRuntimeNeedsCopy(sourceDBPath, mirrorPath)
	if err != nil {
		return false, err
	}
	statePath := portableStoreRefreshStatePath(mirrorPath)
	mirrorCorrupt := false
	if isRepairablePortableSource && !needsCopy {
		mirrorHealthErr := sqliteStoreCachedHealth(ctx, mirrorPath, statePath)
		if mirrorHealthErr != nil {
			if !isSQLiteCorruption(mirrorHealthErr) {
				return false, fmt.Errorf("check portable runtime db: %w", mirrorHealthErr)
			}
			mirrorCorrupt = true
			needsCopy = true
		}
	}
	if needsCopy && isRepairablePortableSource {
		sourceHealthErr := sqliteStoreHealth(ctx, sourceDBPath)
		if sourceHealthErr != nil && isSQLiteCorruption(sourceHealthErr) {
			if err := repairMalformedPortableStoreForDB(ctx, sourceDBPath, configPath); err != nil {
				if !mirrorCorrupt {
					if mirrorHealthErr := sqliteStoreHealth(ctx, mirrorPath); mirrorHealthErr == nil {
						return false, nil
					}
				}
				return false, fmt.Errorf("repair malformed portable store db: %w", err)
			}
			sourceHealthErr = sqliteStoreHealth(ctx, sourceDBPath)
		}
		if sourceHealthErr != nil {
			return false, fmt.Errorf("check portable source db: %w", sourceHealthErr)
		}
	}
	if !needsCopy {
		return false, nil
	}
	if err := copyFileAtomic(sourceDBPath, mirrorPath); err != nil {
		return false, err
	}
	if isRepairablePortableSource {
		_ = markSQLiteStoreHealthVerified(mirrorPath, statePath)
	}
	return true, nil
}

type portableStoreRefreshState struct {
	LastAttempt         string `json:"last_attempt,omitempty"`
	LastSuccess         string `json:"last_success,omitempty"`
	LastFailure         string `json:"last_failure,omitempty"`
	Error               string `json:"error,omitempty"`
	MirrorHealthModTime string `json:"mirror_health_mod_time,omitempty"`
	MirrorHealthSize    int64  `json:"mirror_health_size,omitempty"`
}

func refreshPortableStoreForDBIfDue(ctx context.Context, sourceDBPath, mirrorPath string) error {
	ttl := portableStoreRefreshInterval()
	statePath := portableStoreRefreshStatePath(mirrorPath)
	state := readPortableStoreRefreshState(statePath)
	now := time.Now().UTC()
	if ttl > 0 && recentPortableRefresh(state.LastSuccess, now, ttl) {
		return nil
	}
	if ttl > 0 && recentPortableRefresh(state.LastFailure, now, portableStoreRefreshFailureBackoff) {
		return nil
	}
	lockPath := statePath + ".lock"
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return err
	}
	removeStalePortableRefreshLock(lockPath, now)
	lock, locked := tryGHCommandCacheLock(lockPath)
	if !locked {
		return nil
	}
	defer func() {
		_ = lock.Close()
		_ = os.Remove(lockPath)
	}()
	state = readPortableStoreRefreshState(statePath)
	now = time.Now().UTC()
	if ttl > 0 && recentPortableRefresh(state.LastSuccess, now, ttl) {
		return nil
	}
	state.LastAttempt = now.Format(time.RFC3339Nano)
	err := refreshPortableStoreForDB(ctx, sourceDBPath)
	if err != nil {
		state.LastFailure = time.Now().UTC().Format(time.RFC3339Nano)
		state.Error = err.Error()
		_ = writePortableStoreRefreshState(statePath, state)
		return err
	}
	state.LastSuccess = time.Now().UTC().Format(time.RFC3339Nano)
	state.LastFailure = ""
	state.Error = ""
	return writePortableStoreRefreshState(statePath, state)
}

func removeStalePortableRefreshLock(path string, now time.Time) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if now.Sub(info.ModTime()) <= 2*portableStoreRefreshTimeout {
		return
	}
	_ = os.Remove(path)
}

func portableStoreRefreshInterval() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("GITCRAWL_PORTABLE_REFRESH_TTL")); raw != "" {
		if duration, err := time.ParseDuration(raw); err == nil && duration >= 0 {
			return duration
		}
	}
	return portableStoreRefreshTTL
}

func portableStoreRefreshStatePath(mirrorPath string) string {
	return filepath.Join(filepath.Dir(mirrorPath), ".portable-refresh.json")
}

func readPortableStoreRefreshState(path string) portableStoreRefreshState {
	data, err := os.ReadFile(path)
	if err != nil {
		return portableStoreRefreshState{}
	}
	var state portableStoreRefreshState
	if err := json.Unmarshal(data, &state); err != nil {
		return portableStoreRefreshState{}
	}
	return state
}

func writePortableStoreRefreshState(path string, state portableStoreRefreshState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return writeAtomicFile(path, data, 0o600)
}

func sqliteStoreOpenHealth(ctx context.Context, path string) error {
	if strings.TrimSpace(path) == "" {
		return os.ErrNotExist
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	st, err := store.OpenReadOnly(ctx, path)
	if err != nil {
		return err
	}
	return st.Close()
}

func sqliteStoreCachedHealth(ctx context.Context, path, statePath string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	state := readPortableStoreRefreshState(statePath)
	modTime := info.ModTime().UTC().Format(time.RFC3339Nano)
	if state.MirrorHealthSize == info.Size() && state.MirrorHealthModTime == modTime {
		return sqliteStoreOpenHealth(ctx, path)
	}
	if err := sqliteStoreHealth(ctx, path); err != nil {
		return err
	}
	return markSQLiteStoreHealthVerified(path, statePath)
}

func markSQLiteStoreHealthVerified(path, statePath string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	state := readPortableStoreRefreshState(statePath)
	state.MirrorHealthSize = info.Size()
	state.MirrorHealthModTime = info.ModTime().UTC().Format(time.RFC3339Nano)
	return writePortableStoreRefreshState(statePath, state)
}

func sqliteStoreHealth(ctx context.Context, path string) error {
	st, err := store.OpenReadOnly(ctx, path)
	if err != nil {
		return err
	}
	defer st.Close()
	rows, err := st.DB().QueryContext(ctx, `pragma quick_check`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var problems []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return err
		}
		if strings.TrimSpace(line) != "ok" {
			problems = append(problems, line)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(problems) > 0 {
		return fmt.Errorf("sqlite quick_check failed: %s", strings.Join(problems, "; "))
	}
	return nil
}

func isSQLiteCorruption(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database disk image is malformed") ||
		strings.Contains(message, "file is not a database") ||
		strings.Contains(message, "sqlite quick_check failed") ||
		strings.Contains(message, "sqlite_corrupt") ||
		strings.Contains(message, "error code 11") ||
		strings.Contains(message, "(11)")
}

func preserveMalformedPortableDB(root, dbPath string) error {
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	backupDir := filepath.Join(filepath.Dir(root), "backups", "malformed-"+timestamp)
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return fmt.Errorf("create malformed db backup: %w", err)
	}
	for _, path := range []string{
		dbPath,
		dbPath + "-wal",
		dbPath + "-shm",
		dbPath + ".manifest.json",
	} {
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		target := filepath.Join(backupDir, filepath.Base(path)+".malformed")
		if strings.HasSuffix(path, ".manifest.json") {
			target = filepath.Join(backupDir, filepath.Base(path))
		}
		if err := copyFileAtomic(path, target); err != nil {
			return fmt.Errorf("preserve malformed db evidence: %w", err)
		}
	}
	return nil
}

func recentPortableRefresh(value string, now time.Time, maxAge time.Duration) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return false
	}
	return now.Sub(parsed) <= maxAge
}

func portableRuntimeNeedsCopy(sourceDBPath, mirrorPath string) (bool, error) {
	sourceInfo, err := os.Stat(sourceDBPath)
	if err != nil {
		return false, fmt.Errorf("stat portable source db: %w", err)
	}
	mirrorInfo, err := os.Stat(mirrorPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("stat portable runtime db: %w", err)
	}
	return sourceInfo.ModTime().After(mirrorInfo.ModTime()), nil
}

func copyFileAtomic(sourcePath, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create portable runtime dir: %w", err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open portable source db: %w", err)
	}
	defer source.Close()
	temp, err := os.CreateTemp(filepath.Dir(targetPath), "."+filepath.Base(targetPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create portable runtime temp db: %w", err)
	}
	tempPath := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := io.Copy(temp, source); err != nil {
		_ = temp.Close()
		return fmt.Errorf("copy portable runtime db: %w", err)
	}
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("chmod portable runtime db: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close portable runtime db: %w", err)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		return fmt.Errorf("replace portable runtime db: %w", err)
	}
	cleanup = false
	_ = os.Remove(targetPath + "-wal")
	_ = os.Remove(targetPath + "-shm")
	return nil
}

func portableStoreRoot(dbPath string) (string, bool) {
	dir := filepath.Clean(filepath.Dir(dbPath))
	for {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func portableStoreIsGitWorktree(ctx context.Context, dir string) bool {
	out, err := gitOutput(ctx, "", "-C", dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

func portableStoreRepairAllowed(root, configPath string) bool {
	if strings.TrimSpace(root) == "" {
		return false
	}
	if info, err := os.Stat(filepath.Join(root, ".git", "info", portableStoreMarkerFile)); err == nil && !info.IsDir() {
		return true
	}
	defaultStoresDir := filepath.Join(filepath.Dir(config.ResolvePath(configPath)), "stores")
	rel, err := filepath.Rel(defaultStoresDir, root)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}

func gitWorktreeClean(ctx context.Context, dir string) bool {
	if err := runGit(ctx, "", "-C", dir, "update-index", "-q", "--refresh"); err != nil {
		return false
	}
	if err := runGit(ctx, "", "-C", dir, "diff", "--quiet", "--"); err != nil {
		return false
	}
	if err := runGit(ctx, "", "-C", dir, "diff", "--cached", "--quiet", "--"); err != nil {
		return false
	}
	return true
}
