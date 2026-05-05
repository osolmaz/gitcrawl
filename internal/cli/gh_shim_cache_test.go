package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/openclaw/gitcrawl/internal/config"
	"github.com/openclaw/gitcrawl/internal/store"
)

func TestGHShimCachesReadOnlyFallbackCommands(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)
	dir := t.TempDir()
	countPath := filepath.Join(dir, "count")
	ghPath := filepath.Join(dir, "gh")
	script := `#!/bin/sh
count=0
if [ -f "$GH_SHIM_COUNT" ]; then
  count=$(cat "$GH_SHIM_COUNT")
fi
count=$((count + 1))
printf "%s" "$count" > "$GH_SHIM_COUNT"
echo "call-$count:$*"
`
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("GITCRAWL_GH_PATH", ghPath)
	t.Setenv("GH_SHIM_COUNT", countPath)
	t.Setenv("GH_REPO", "cache-test/"+filepath.Base(dir))
	t.Setenv("GITCRAWL_GH_CACHE_TTL", "1m")

	run := New()
	var stdout bytes.Buffer
	run.Stdout = &stdout
	args := []string{"--config", configPath, "gh", "run", "view", "123", "-R", "openclaw/openclaw", "--json", "status"}
	if err := run.Run(ctx, args); err != nil {
		t.Fatalf("first cached read: %v", err)
	}
	first := stdout.String()
	stdout.Reset()
	if err := run.Run(ctx, args); err != nil {
		t.Fatalf("second cached read: %v", err)
	}
	if second := stdout.String(); second != first {
		t.Fatalf("cached output changed: first=%q second=%q", first, second)
	}
	countData, err := os.ReadFile(countPath)
	if err != nil {
		t.Fatalf("read count: %v", err)
	}
	if strings.TrimSpace(string(countData)) != "1" {
		t.Fatalf("fake gh call count = %q, want 1", countData)
	}

	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "xcache", "stats", "--json"}); err != nil {
		t.Fatalf("xcache stats: %v", err)
	}
	var stats map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &stats); err != nil {
		t.Fatalf("decode stats: %v\n%s", err, stdout.String())
	}
	if int(stats["entries"].(float64)) != 1 {
		t.Fatalf("stats = %#v", stats)
	}
	counters := stats["counters"].(map[string]any)
	if int(counters["backend_misses"].(float64)) != 1 || int(counters["fallback_hits"].(float64)) != 1 {
		t.Fatalf("counters = %#v", counters)
	}

	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "xcache", "keys", "--json"}); err != nil {
		t.Fatalf("xcache keys: %v", err)
	}
	var keys []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &keys); err != nil {
		t.Fatalf("decode keys: %v\n%s", err, stdout.String())
	}
	if len(keys) != 1 || keys[0]["command"] != "run view" {
		t.Fatalf("keys = %#v", keys)
	}

	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "xcache", "flush", "--json"}); err != nil {
		t.Fatalf("xcache flush: %v", err)
	}
	var flushed map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &flushed); err != nil {
		t.Fatalf("decode flush: %v\n%s", err, stdout.String())
	}
	if int(flushed["removed"].(float64)) != 1 {
		t.Fatalf("flushed = %#v", flushed)
	}
}

func TestGHShimCachesGHXStyleReadOnlyFallbackCommands(t *testing.T) {
	for _, args := range [][]string{
		{"gh", "release", "view", "v1.2.3", "-R", "openclaw/openclaw"},
		{"gh", "workflow", "view", "ci.yml", "-R", "openclaw/openclaw"},
		{"gh", "secret", "list", "-R", "openclaw/openclaw"},
		{"gh", "variable", "list", "-R", "openclaw/openclaw"},
		{"gh", "ruleset", "list", "-R", "openclaw/openclaw"},
	} {
		if !cacheableGHRead(args[1:]) {
			t.Fatalf("%v should be cacheable", args)
		}
	}
}

func TestGHShimCachesReadOnlyFallbackErrors(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)
	dir := t.TempDir()
	countPath := filepath.Join(dir, "count")
	ghPath := filepath.Join(dir, "gh")
	script := `#!/bin/sh
count=0
if [ -f "$GH_SHIM_COUNT" ]; then
  count=$(cat "$GH_SHIM_COUNT")
fi
count=$((count + 1))
printf "%s" "$count" > "$GH_SHIM_COUNT"
echo "missing-$count:$*" >&2
exit 42
`
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("GITCRAWL_GH_PATH", ghPath)
	t.Setenv("GH_SHIM_COUNT", countPath)
	t.Setenv("GH_REPO", "error-cache/"+filepath.Base(dir))
	t.Setenv("GITCRAWL_GH_CACHE_TTL", "1m")

	args := []string{"--config", configPath, "gh", "release", "view", "missing", "-R", "openclaw/openclaw"}
	for i := 0; i < 2; i++ {
		run := New()
		var stderr bytes.Buffer
		run.Stderr = &stderr
		err := run.Run(ctx, args)
		if err == nil {
			t.Fatalf("run %d unexpectedly succeeded", i)
		}
		if !strings.Contains(stderr.String(), "missing-1:release view missing") {
			t.Fatalf("stderr %d = %q", i, stderr.String())
		}
	}
	countData, err := os.ReadFile(countPath)
	if err != nil {
		t.Fatalf("read count: %v", err)
	}
	if strings.TrimSpace(string(countData)) != "1" {
		t.Fatalf("fake gh call count = %q, want 1", countData)
	}
}

func TestGHShimMutatingFallbackClearsCacheForGHXStyleMutations(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)
	dir := t.TempDir()
	countPath := filepath.Join(dir, "count")
	ghPath := filepath.Join(dir, "gh")
	script := `#!/bin/sh
count=0
if [ -f "$GH_SHIM_COUNT" ]; then
  count=$(cat "$GH_SHIM_COUNT")
fi
count=$((count + 1))
printf "%s" "$count" > "$GH_SHIM_COUNT"
echo "call-$count:$*"
`
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("GITCRAWL_GH_PATH", ghPath)
	t.Setenv("GH_SHIM_COUNT", countPath)
	t.Setenv("GH_REPO", "mutation-cache/"+filepath.Base(dir))
	t.Setenv("GITCRAWL_GH_CACHE_TTL", "1m")

	run := New()
	var stdout bytes.Buffer
	run.Stdout = &stdout
	readArgs := []string{"--config", configPath, "gh", "release", "view", "v1", "-R", "openclaw/openclaw"}
	if err := run.Run(ctx, readArgs); err != nil {
		t.Fatalf("first read: %v", err)
	}
	stdout.Reset()
	if err := run.Run(ctx, readArgs); err != nil {
		t.Fatalf("second read: %v", err)
	}
	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "run", "rerun", "123", "-R", "openclaw/openclaw"}); err != nil {
		t.Fatalf("mutation: %v", err)
	}
	stdout.Reset()
	if err := run.Run(ctx, readArgs); err != nil {
		t.Fatalf("third read: %v", err)
	}
	countData, err := os.ReadFile(countPath)
	if err != nil {
		t.Fatalf("read count: %v", err)
	}
	if strings.TrimSpace(string(countData)) != "3" {
		t.Fatalf("fake gh call count = %q, want 3", countData)
	}
}

func TestGHShimCachesPRDiffByHeadSHA(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)
	dir := t.TempDir()
	countPath := filepath.Join(dir, "count")
	ghPath := filepath.Join(dir, "gh")
	script := `#!/bin/sh
count=0
if [ -f "$GH_SHIM_COUNT" ]; then
  count=$(cat "$GH_SHIM_COUNT")
fi
count=$((count + 1))
printf "%s" "$count" > "$GH_SHIM_COUNT"
echo "diff-$count:$*"
`
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("GITCRAWL_GH_PATH", ghPath)
	t.Setenv("GH_SHIM_COUNT", countPath)

	run := New()
	var stdout bytes.Buffer
	run.Stdout = &stdout
	args := []string{"--config", configPath, "gh", "pr", "diff", "12", "-R", "openclaw/openclaw"}
	if err := run.Run(ctx, args); err != nil {
		t.Fatalf("first pr diff: %v", err)
	}
	stdout.Reset()
	if err := run.Run(ctx, args); err != nil {
		t.Fatalf("second pr diff: %v", err)
	}
	countData, err := os.ReadFile(countPath)
	if err != nil {
		t.Fatalf("read count: %v", err)
	}
	if strings.TrimSpace(string(countData)) != "1" {
		t.Fatalf("fake gh call count = %q, want 1", countData)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	repo, err := st.RepositoryByFullName(ctx, "openclaw/openclaw")
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	if _, err := st.UpsertThread(ctx, store.Thread{
		RepoID: repo.ID, GitHubID: "12", Number: 12, Kind: "pull_request", State: "open",
		Title: "Manifest cache update", AuthorLogin: "bob", AuthorType: "User",
		HTMLURL: "https://github.com/openclaw/openclaw/pull/12", LabelsJSON: "[]", AssigneesJSON: "[]",
		RawJSON: `{"head":{"sha":"def456"}}`, ContentHash: "pr-12-new", IsDraft: true,
		UpdatedAtGitHub: "2026-04-27T03:00:00Z", UpdatedAt: "2026-04-27T03:00:00Z",
	}); err != nil {
		t.Fatalf("update pr head: %v", err)
	}
	if err := st.UpsertPullRequestCache(ctx, store.PullRequestDetail{
		ThreadID:  prIDForTest(t, ctx, st, repo.ID, 12),
		RepoID:    repo.ID,
		Number:    12,
		HeadSHA:   "def456",
		RawJSON:   `{"head":{"sha":"def456"}}`,
		FetchedAt: "2026-04-27T03:00:00Z",
		UpdatedAt: "2026-04-27T03:00:00Z",
	}, nil, nil, nil, nil); err != nil {
		t.Fatalf("update pr cache head: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	stdout.Reset()
	if err := run.Run(ctx, args); err != nil {
		t.Fatalf("third pr diff after head change: %v", err)
	}
	countData, err = os.ReadFile(countPath)
	if err != nil {
		t.Fatalf("read count after update: %v", err)
	}
	if strings.TrimSpace(string(countData)) != "2" {
		t.Fatalf("fake gh call count after head update = %q, want 2", countData)
	}
}

func TestGHShimXCacheGCRemovesExpiredEntries(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)
	dir := t.TempDir()
	ghPath := filepath.Join(dir, "gh")
	if err := os.WriteFile(ghPath, []byte("#!/bin/sh\necho cached:$*\n"), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("GITCRAWL_GH_PATH", ghPath)
	t.Setenv("GITCRAWL_GH_CACHE_TTL", "1ns")

	run := New()
	var stdout bytes.Buffer
	run.Stdout = &stdout
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "run", "view", "789", "-R", "openclaw/openclaw"}); err != nil {
		t.Fatalf("cached read: %v", err)
	}
	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "xcache", "gc", "--json"}); err != nil {
		t.Fatalf("xcache gc: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode gc: %v\n%s", err, stdout.String())
	}
	if int(result["removed"].(float64)) != 1 {
		t.Fatalf("gc = %#v", result)
	}
}

func TestGHShimCoalescesConcurrentReadOnlyFallbacks(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)
	dir := t.TempDir()
	countPath := filepath.Join(dir, "count")
	ghPath := filepath.Join(dir, "gh")
	script := `#!/bin/sh
count=0
if [ -f "$GH_SHIM_COUNT" ]; then
  count=$(cat "$GH_SHIM_COUNT")
fi
count=$((count + 1))
printf "%s" "$count" > "$GH_SHIM_COUNT"
sleep 0.2
echo "call-$count:$*"
`
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("GITCRAWL_GH_PATH", ghPath)
	t.Setenv("GH_SHIM_COUNT", countPath)
	t.Setenv("GH_REPO", "coalesce-test/"+filepath.Base(dir))
	t.Setenv("GITCRAWL_GH_CACHE_TTL", "1m")

	args := []string{"--config", configPath, "gh", "run", "view", "456", "-R", "openclaw/openclaw", "--json", "status"}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	outputs := make(chan string, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			run := New()
			var stdout bytes.Buffer
			run.Stdout = &stdout
			if err := run.Run(ctx, args); err != nil {
				errs <- err
				return
			}
			outputs <- stdout.String()
		}()
	}
	wg.Wait()
	close(errs)
	close(outputs)
	for err := range errs {
		t.Fatalf("coalesced run: %v", err)
	}
	if len(outputs) != 2 {
		t.Fatalf("outputs = %d, want 2", len(outputs))
	}
	var first string
	for out := range outputs {
		if first == "" {
			first = out
		} else if out != first {
			t.Fatalf("coalesced outputs differ: %q vs %q", first, out)
		}
	}
	countData, err := os.ReadFile(countPath)
	if err != nil {
		t.Fatalf("read count: %v", err)
	}
	if strings.TrimSpace(string(countData)) != "1" {
		t.Fatalf("fake gh call count = %q, want 1", countData)
	}
}
