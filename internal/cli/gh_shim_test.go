package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	fetchedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if err := st.UpsertPullRequestCache(ctx, store.PullRequestDetail{
		ThreadID:         prID,
		RepoID:           repoID,
		Number:           12,
		BaseSHA:          "base123",
		HeadSHA:          "abc123",
		HeadRef:          "manifest-cache",
		HeadRepoFullName: "openclaw/openclaw",
		MergeableState:   "clean",
		Additions:        10,
		Deletions:        2,
		ChangedFiles:     1,
		RawJSON:          `{"head":{"sha":"abc123"}}`,
		FetchedAt:        fetchedAt,
		UpdatedAt:        fetchedAt,
	}, []store.PullRequestFile{{
		ThreadID:  prID,
		Path:      "internal/cache.go",
		Status:    "modified",
		Additions: 10,
		Deletions: 2,
		Changes:   12,
		RawJSON:   "{}",
		FetchedAt: fetchedAt,
	}}, []store.PullRequestCommit{{
		ThreadID:    prID,
		SHA:         "commit123",
		Message:     "feat: cache",
		AuthorLogin: "alice",
		AuthorName:  "Alice",
		CommittedAt: "2026-04-27T01:00:00Z",
		HTMLURL:     "https://github.com/openclaw/openclaw/commit/commit123",
		RawJSON:     "{}",
		FetchedAt:   fetchedAt,
	}}, []store.PullRequestCheck{{
		ThreadID:     prID,
		Name:         "test",
		Status:       "completed",
		Conclusion:   "success",
		DetailsURL:   "https://github.com/openclaw/openclaw/actions/runs/99",
		WorkflowName: "CI",
		RawJSON:      "{}",
		FetchedAt:    fetchedAt,
	}}, []store.WorkflowRun{{
		RepoID:       repoID,
		RunID:        "99",
		RunNumber:    7,
		HeadBranch:   "manifest-cache",
		HeadSHA:      "abc123",
		Status:       "completed",
		Conclusion:   "success",
		WorkflowName: "CI",
		Event:        "pull_request",
		HTMLURL:      "https://github.com/openclaw/openclaw/actions/runs/99",
		CreatedAtGH:  "2026-04-27T01:00:00Z",
		UpdatedAtGH:  "2026-04-27T02:00:00Z",
		RawJSON:      "{}",
		FetchedAt:    fetchedAt,
	}}); err != nil {
		t.Fatalf("seed pr cache: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	return configPath
}
