package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPortableRuntimeUtilityBranches(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.db")
	mirror := filepath.Join(dir, "runtime", "source.db")
	if _, err := portableRuntimeNeedsCopy(source, mirror); err == nil {
		t.Fatal("missing source should fail")
	}
	if err := os.WriteFile(source, []byte("v1"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	needs, err := portableRuntimeNeedsCopy(source, mirror)
	if err != nil || !needs {
		t.Fatalf("missing mirror needs copy=%v err=%v", needs, err)
	}
	if err := copyFileAtomic(source, mirror); err != nil {
		t.Fatalf("copy mirror: %v", err)
	}
	if err := os.WriteFile(mirror+"-wal", []byte("wal"), 0o644); err != nil {
		t.Fatalf("write wal: %v", err)
	}
	if err := os.WriteFile(mirror+"-shm", []byte("shm"), 0o644); err != nil {
		t.Fatalf("write shm: %v", err)
	}
	if err := os.Chtimes(mirror, time.Now().Add(time.Hour), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("age mirror: %v", err)
	}
	needs, err = portableRuntimeNeedsCopy(source, mirror)
	if err != nil || needs {
		t.Fatalf("fresh mirror needs copy=%v err=%v", needs, err)
	}
	if err := copyFileAtomic(source, mirror); err != nil {
		t.Fatalf("recopy mirror: %v", err)
	}
	if _, err := os.Stat(mirror + "-wal"); !os.IsNotExist(err) {
		t.Fatalf("wal sidecar should be removed, err=%v", err)
	}
	if _, err := os.Stat(mirror + "-shm"); !os.IsNotExist(err) {
		t.Fatalf("shm sidecar should be removed, err=%v", err)
	}

	statePath := portableStoreRefreshStatePath(mirror)
	state := portableStoreRefreshState{LastAttempt: "attempt", LastSuccess: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := writePortableStoreRefreshState(statePath, state); err != nil {
		t.Fatalf("write state: %v", err)
	}
	if got := readPortableStoreRefreshState(statePath); got.LastAttempt != "attempt" || got.LastSuccess == "" {
		t.Fatalf("state = %+v", got)
	}
	if err := os.WriteFile(statePath, []byte("{"), 0o600); err != nil {
		t.Fatalf("write invalid state: %v", err)
	}
	if got := readPortableStoreRefreshState(statePath); got.LastAttempt != "" {
		t.Fatalf("invalid state should decode empty, got %+v", got)
	}
	now := time.Now().UTC()
	if recentPortableRefresh("", now, time.Minute) || recentPortableRefresh("bad", now, time.Minute) || !recentPortableRefresh(now.Format(time.RFC3339Nano), now, time.Minute) {
		t.Fatal("recent refresh classification mismatch")
	}
	lockPath := filepath.Join(dir, "refresh.lock")
	if err := os.WriteFile(lockPath, []byte("123\n"), 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	removeStalePortableRefreshLock(lockPath, now)
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("fresh lock should remain: %v", err)
	}
	old := now.Add(-3 * portableStoreRefreshTimeout)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatalf("age lock: %v", err)
	}
	removeStalePortableRefreshLock(lockPath, now)
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("stale lock should be removed, err=%v", err)
	}
	t.Setenv("GITCRAWL_PORTABLE_REFRESH_TTL", "0")
	if got := portableStoreRefreshInterval(); got != 0 {
		t.Fatalf("zero ttl = %s", got)
	}
	t.Setenv("GITCRAWL_PORTABLE_REFRESH_TTL", "bad")
	if got := portableStoreRefreshInterval(); got != portableStoreRefreshTTL {
		t.Fatalf("bad ttl fallback = %s", got)
	}
	if err := refreshPortableStoreForDB(context.Background(), source); err != nil {
		t.Fatalf("non-portable refresh should be no-op: %v", err)
	}
	configDir := filepath.Join(dir, "config-root")
	t.Setenv("GITCRAWL_CONFIG", filepath.Join(configDir, "config.toml"))
	defaultStore := filepath.Join(configDir, "stores", "gitcrawl-store")
	if err := os.MkdirAll(defaultStore, 0o755); err != nil {
		t.Fatalf("mkdir default store: %v", err)
	}
	if !portableStoreRepairAllowed(defaultStore, "") {
		t.Fatal("default portable store should be repairable")
	}
	if portableStoreRepairAllowed(configDir, "") {
		t.Fatal("config root should not be repairable without marker")
	}
	markedStore := filepath.Join(dir, "custom-store")
	markedInfo := filepath.Join(markedStore, ".git", "info")
	if err := os.MkdirAll(markedInfo, 0o755); err != nil {
		t.Fatalf("mkdir marked store info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(markedInfo, portableStoreMarkerFile), []byte("gitcrawl portable store\n"), 0o644); err != nil {
		t.Fatalf("write marked store marker: %v", err)
	}
	if !portableStoreRepairAllowed(markedStore, "") {
		t.Fatal("marked portable store should be repairable")
	}
	explicitConfigDir := filepath.Join(dir, "explicit-config-root")
	explicitConfigPath := filepath.Join(explicitConfigDir, "nested", "config.toml")
	explicitStore := filepath.Join(explicitConfigDir, "nested", "stores", "gitcrawl-store")
	if err := os.MkdirAll(explicitStore, 0o755); err != nil {
		t.Fatalf("mkdir explicit default store: %v", err)
	}
	if !portableStoreRepairAllowed(explicitStore, explicitConfigPath) {
		t.Fatal("explicit-config default portable store should be repairable")
	}
}
