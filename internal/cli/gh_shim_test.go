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

func TestGHShimSearchAcceptsGHFlags(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)

	run := New()
	var stdout bytes.Buffer
	run.Stdout = &stdout
	if err := run.Run(ctx, []string{
		"--config", configPath,
		"gh", "search", "issues", "hot loop",
		"-R", "openclaw/openclaw",
		"--state", "open",
		"--match", "comments",
		"--sort", "updated",
		"--order", "desc",
		"--json", "number,title,state,url",
		"--limit", "10",
	}); err != nil {
		t.Fatalf("gh shim search: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("decode search: %v\n%s", err, stdout.String())
	}
	if len(rows) != 1 || int(rows[0]["number"].(float64)) != 10 {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestGHShimViewAndListUseLocalCache(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)

	run := New()
	var stdout bytes.Buffer
	run.Stdout = &stdout
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "pr", "view", "12", "-R", "openclaw/openclaw", "--json", "number,title,isDraft,author"}); err != nil {
		t.Fatalf("gh pr view: %v", err)
	}
	var view map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &view); err != nil {
		t.Fatalf("decode view: %v\n%s", err, stdout.String())
	}
	if int(view["number"].(float64)) != 12 || view["isDraft"] != true {
		t.Fatalf("view = %#v", view)
	}

	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "issue", "list", "-R", "openclaw/openclaw", "--state", "open", "--json", "number,title"}); err != nil {
		t.Fatalf("gh issue list: %v", err)
	}
	var list []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v\n%s", err, stdout.String())
	}
	if len(list) != 1 || int(list[0]["number"].(float64)) != 10 {
		t.Fatalf("list = %#v", list)
	}

	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "issue", "list", "-R", "openclaw/openclaw", "--author", "alice", "--assignee", "peter", "--label", "bug", "--json", "number,title"}); err != nil {
		t.Fatalf("gh issue list filtered: %v", err)
	}
	if err := json.Unmarshal(stdout.Bytes(), &list); err != nil {
		t.Fatalf("decode filtered list: %v\n%s", err, stdout.String())
	}
	if len(list) != 1 || int(list[0]["number"].(float64)) != 10 {
		t.Fatalf("filtered list = %#v", list)
	}
}

func TestGHShimFallsBackForUnsupportedRead(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)
	dir := t.TempDir()
	ghPath := filepath.Join(dir, "gh")
	if err := os.WriteFile(ghPath, []byte("#!/bin/sh\necho fallback:$*\n"), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("GITCRAWL_GH_PATH", ghPath)

	run := New()
	var stdout bytes.Buffer
	run.Stdout = &stdout
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "pr", "view", "12", "-R", "openclaw/openclaw", "--json", "unsupportedField"}); err != nil {
		t.Fatalf("fallback: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "fallback:pr view 12 -R openclaw/openclaw --json unsupportedField" {
		t.Fatalf("fallback output = %q", got)
	}
}

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

func seedGHShimRepo(t *testing.T, ctx context.Context) string {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	dbPath := filepath.Join(dir, "gitcrawl.db")
	app := New()
	if err := app.Run(ctx, []string{"--config", configPath, "init", "--db", dbPath}); err != nil {
		t.Fatalf("init: %v", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.CacheDir = filepath.Join(dir, "cache")
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	repoID, err := st.UpsertRepository(ctx, store.Repository{
		Owner:     "openclaw",
		Name:      "openclaw",
		FullName:  "openclaw/openclaw",
		RawJSON:   "{}",
		UpdatedAt: "2026-04-27T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("seed repository: %v", err)
	}
	issueID, err := st.UpsertThread(ctx, store.Thread{
		RepoID:          repoID,
		GitHubID:        "10",
		Number:          10,
		Kind:            "issue",
		State:           "open",
		Title:           "Hot loop burns CPU",
		Body:            "the runtime has a hot loop",
		AuthorLogin:     "alice",
		AuthorType:      "User",
		HTMLURL:         "https://github.com/openclaw/openclaw/issues/10",
		LabelsJSON:      `[{"name":"bug","color":"d73a4a"}]`,
		AssigneesJSON:   `[{"login":"peter"}]`,
		RawJSON:         "{}",
		ContentHash:     "issue-10",
		UpdatedAtGitHub: "2026-04-27T01:00:00Z",
		UpdatedAt:       "2026-04-27T01:00:00Z",
	})
	if err != nil {
		t.Fatalf("seed issue: %v", err)
	}
	if _, err := st.UpsertDocument(ctx, store.Document{ThreadID: issueID, Title: "Hot loop burns CPU", RawText: "runtime hot loop burns CPU", DedupeText: "runtime hot loop burns cpu", UpdatedAt: "2026-04-27T01:00:00Z"}); err != nil {
		t.Fatalf("seed issue document: %v", err)
	}
	prID, err := st.UpsertThread(ctx, store.Thread{
		RepoID:          repoID,
		GitHubID:        "12",
		Number:          12,
		Kind:            "pull_request",
		State:           "open",
		Title:           "Manifest cache update",
		AuthorLogin:     "bob",
		AuthorType:      "User",
		HTMLURL:         "https://github.com/openclaw/openclaw/pull/12",
		LabelsJSON:      "[]",
		AssigneesJSON:   "[]",
		RawJSON:         `{"head":{"sha":"abc123"}}`,
		ContentHash:     "pr-12",
		IsDraft:         true,
		UpdatedAtGitHub: "2026-04-27T02:00:00Z",
		UpdatedAt:       "2026-04-27T02:00:00Z",
	})
	if err != nil {
		t.Fatalf("seed pr: %v", err)
	}
	if _, err := st.UpsertDocument(ctx, store.Document{ThreadID: prID, Title: "Manifest cache update", RawText: "manifest cache refresh", DedupeText: "manifest cache refresh", UpdatedAt: "2026-04-27T02:00:00Z"}); err != nil {
		t.Fatalf("seed pr document: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	return configPath
}
