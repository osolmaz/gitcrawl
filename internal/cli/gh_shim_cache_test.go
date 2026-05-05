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
	"time"

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

func TestGHShimCommandAwareCacheTTLs(t *testing.T) {
	t.Setenv("GITCRAWL_GH_CACHE_TTL", "")
	if got := ghCommandCacheTTL([]string{"api", "users/octocat"}); got != 7*24*time.Hour {
		t.Fatalf("user api ttl = %s, want 7d", got)
	}
	if got := ghCommandCacheTTL([]string{"api", "graphql", "-f", "query={ viewer { login } }"}); got != 6*time.Hour {
		t.Fatalf("graphql api ttl = %s, want 6h", got)
	}
	if got := ghCommandCacheTTL([]string{"run", "view", "123", "--log"}); got != 12*time.Hour {
		t.Fatalf("run log ttl = %s, want 12h", got)
	}
	if got := ghCommandCacheTTL([]string{"run", "view", "123", "--job", "456"}); got != 5*time.Minute {
		t.Fatalf("run job ttl = %s, want 5m", got)
	}
	if got := ghCommandCacheTTL([]string{"run", "list", "-R", "openclaw/openclaw"}); got != 2*time.Minute {
		t.Fatalf("run list ttl = %s, want 2m", got)
	}
	if got := ghCommandCacheTTL([]string{"search", "issues", "cache"}); got != 15*time.Minute {
		t.Fatalf("search ttl = %s, want 15m", got)
	}
	if got := ghCommandCacheTTL([]string{"api", "-i", "repos/openclaw/openclaw/actions/runs/123/logs"}); got != 12*time.Hour {
		t.Fatalf("actions log api ttl = %s, want 12h", got)
	}
	if got := ghCommandCacheTTL([]string{"api", "repos/openclaw/openclaw/actions/runs/123"}); got != 2*time.Minute {
		t.Fatalf("actions run api ttl = %s, want 2m", got)
	}
	if got := normalizeGHAPIRoute([]string{"repos/openclaw/openclaw/actions/runs?per_page=1"}); got != "api repos/:owner/:repo/actions/runs" {
		t.Fatalf("normalized actions route = %q", got)
	}
	if got := normalizeGHAPIRoute([]string{"--paginate", "repos/openclaw/openclaw/issues?state=all&creator=octocat", "--jq", ".[].number"}); got != "api repos/:owner/:repo/issues" {
		t.Fatalf("normalized paginated issues route = %q", got)
	}
	entry := ghCommandCacheEntry{CreatedAt: time.Now().Add(-3 * time.Minute), ExitCode: 1, Stderr: "HTTP 403: API rate limit exceeded"}
	if ttl := ghCommandCacheEntryTTL(entry, 12*time.Hour); ttl != 2*time.Minute {
		t.Fatalf("rate-limit error ttl = %s, want 2m", ttl)
	}
}

func TestGHShimGraphQLReadOnlyDetection(t *testing.T) {
	if !cacheableGHRead([]string{"api", "graphql", "-f", "login=octocat", "-f", "query=query { viewer { login } }"}) {
		t.Fatalf("graphql query should be cacheable")
	}
	if !cacheableGHRead([]string{"api", "graphql", "-f", "query={ viewer { login } }"}) {
		t.Fatalf("anonymous graphql query should be cacheable")
	}
	if cacheableGHRead([]string{"api", "graphql", "-f", "query=mutation { addStar(input:{starrableId:\"x\"}) { clientMutationId } }"}) {
		t.Fatalf("graphql mutation should not be cacheable")
	}
	if cacheableGHRead([]string{"api", "graphql", "-X", "PATCH", "-f", "query={ viewer { login } }"}) {
		t.Fatalf("graphql non-read method should not be cacheable")
	}
	if cacheableGHRead([]string{"api", "graphql", "-f", "query=@query.graphql"}) {
		t.Fatalf("graphql file-backed query should not be cacheable")
	}
	if cacheableGHRead([]string{"api", "repos/openclaw/openclaw/issues", "-f", "title=x"}) {
		t.Fatalf("REST API fields should not be cacheable")
	}
}

func TestGHShimExplicitCacheKeysAreCwdIndependent(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)
	a := New()
	a.configPath = configPath
	t.Setenv("GH_REPO", "")
	t.Setenv("GH_HOST", "")

	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(original) }()
	firstDir := t.TempDir()
	secondDir := t.TempDir()

	if err := os.Chdir(firstDir); err != nil {
		t.Fatalf("chdir first: %v", err)
	}
	apiFirst := a.ghCommandCacheKey(ctx, []string{"api", "users/octocat"})
	repoFirst := a.ghCommandCacheKey(ctx, []string{"repo", "view", "openclaw/gitcrawl", "--json", "nameWithOwner"})
	runFirst := a.ghCommandCacheKey(ctx, []string{"run", "view", "123", "-R", "openclaw/gitcrawl", "--json", "status"})
	implicitFirst := a.ghCommandCacheKey(ctx, []string{"repo", "view", "--json", "nameWithOwner"})

	if err := os.Chdir(secondDir); err != nil {
		t.Fatalf("chdir second: %v", err)
	}
	if apiSecond := a.ghCommandCacheKey(ctx, []string{"api", "users/octocat"}); apiSecond != apiFirst {
		t.Fatalf("explicit api key changed across cwd: %s != %s", apiSecond, apiFirst)
	}
	if repoSecond := a.ghCommandCacheKey(ctx, []string{"repo", "view", "openclaw/gitcrawl", "--json", "nameWithOwner"}); repoSecond != repoFirst {
		t.Fatalf("explicit repo key changed across cwd: %s != %s", repoSecond, repoFirst)
	}
	if runSecond := a.ghCommandCacheKey(ctx, []string{"run", "view", "123", "-R", "openclaw/gitcrawl", "--json", "status"}); runSecond != runFirst {
		t.Fatalf("explicit -R key changed across cwd: %s != %s", runSecond, runFirst)
	}
	if implicitSecond := a.ghCommandCacheKey(ctx, []string{"repo", "view", "--json", "nameWithOwner"}); implicitSecond == implicitFirst {
		t.Fatalf("implicit repo key did not include cwd")
	}

	if err := os.Setenv("GH_REPO", "openclaw/other"); err != nil {
		t.Fatalf("set GH_REPO: %v", err)
	}
	if apiWithEnv := a.ghCommandCacheKey(ctx, []string{"api", "users/octocat"}); apiWithEnv != apiFirst {
		t.Fatalf("explicit api key changed across GH_REPO: %s != %s", apiWithEnv, apiFirst)
	}
	if repoWithEnv := a.ghCommandCacheKey(ctx, []string{"repo", "view", "openclaw/gitcrawl", "--json", "nameWithOwner"}); repoWithEnv != repoFirst {
		t.Fatalf("explicit repo key changed across GH_REPO: %s != %s", repoWithEnv, repoFirst)
	}
	if runWithEnv := a.ghCommandCacheKey(ctx, []string{"run", "view", "123", "-R", "openclaw/gitcrawl", "--json", "status"}); runWithEnv != runFirst {
		t.Fatalf("explicit -R key changed across GH_REPO: %s != %s", runWithEnv, runFirst)
	}
}

func TestGHShimGHRepoScopedCacheKeysAreCwdIndependent(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)
	a := New()
	a.configPath = configPath
	t.Setenv("GH_HOST", "")
	t.Setenv("GH_REPO", "")

	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(original) }()

	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir first: %v", err)
	}
	if err := os.Setenv("GH_REPO", "openclaw/one"); err != nil {
		t.Fatalf("set GH_REPO one: %v", err)
	}
	first := a.ghCommandCacheKey(ctx, []string{"repo", "view", "--json", "nameWithOwner"})

	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir second: %v", err)
	}
	second := a.ghCommandCacheKey(ctx, []string{"repo", "view", "--json", "nameWithOwner"})
	if second != first {
		t.Fatalf("GH_REPO-scoped key changed across cwd: %s != %s", second, first)
	}

	if err := os.Setenv("GH_REPO", "openclaw/two"); err != nil {
		t.Fatalf("set GH_REPO two: %v", err)
	}
	if otherRepo := a.ghCommandCacheKey(ctx, []string{"repo", "view", "--json", "nameWithOwner"}); otherRepo == first {
		t.Fatalf("GH_REPO-scoped key ignored GH_REPO change")
	}
}

func TestGHShimTracksBackendMissesByCommandAndRoute(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)
	dir := t.TempDir()
	ghPath := filepath.Join(dir, "gh")
	if err := os.WriteFile(ghPath, []byte("#!/bin/sh\necho api:$*\n"), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("GITCRAWL_GH_PATH", ghPath)
	t.Setenv("GH_REPO", "miss-test/"+filepath.Base(dir))
	t.Setenv("GITCRAWL_GH_CACHE_TTL", "1m")

	run := New()
	var stdout bytes.Buffer
	run.Stdout = &stdout
	args := []string{"--config", configPath, "gh", "api", "-i", "repos/openclaw/openclaw/actions/runs/123/logs"}
	if err := run.Run(ctx, args); err != nil {
		t.Fatalf("api read: %v", err)
	}
	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "xcache", "stats", "--json"}); err != nil {
		t.Fatalf("xcache stats: %v", err)
	}
	var stats ghCommandCacheStats
	if err := json.Unmarshal(stdout.Bytes(), &stats); err != nil {
		t.Fatalf("decode stats: %v\n%s", err, stdout.String())
	}
	if stats.Counters.BackendMissesByCommand["api"] != 1 {
		t.Fatalf("backend misses by command = %#v", stats.Counters.BackendMissesByCommand)
	}
	if stats.Counters.BackendMissesByRoute["api repos/:owner/:repo/actions/runs/:id/logs"] != 1 {
		t.Fatalf("backend misses by route = %#v", stats.Counters.BackendMissesByRoute)
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
