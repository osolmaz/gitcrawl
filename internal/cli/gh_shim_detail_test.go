package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openclaw/gitcrawl/internal/config"
	"github.com/openclaw/gitcrawl/internal/store"
)

func TestGHShimViewAndListUseLocalCache(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)

	run := New()
	var stdout bytes.Buffer
	run.Stdout = &stdout
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "pr", "view", "12", "-R", "openclaw/openclaw", "--json", "number,title,isDraft,author,comments,assignees,baseRefName,maintainerCanModify,mergeCommit"}); err != nil {
		t.Fatalf("gh pr view: %v", err)
	}
	var view map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &view); err != nil {
		t.Fatalf("decode view: %v\n%s", err, stdout.String())
	}
	comments := view["comments"].([]any)
	if int(view["number"].(float64)) != 12 || view["isDraft"] != true || len(comments) != 1 || comments[0].(map[string]any)["body"] != "cache path looks good" || view["baseRefName"] != "main" || view["maintainerCanModify"] != true {
		t.Fatalf("view = %#v", view)
	}
	if view["mergeCommit"] != nil {
		t.Fatalf("mergeCommit = %#v, want nil for open PR", view["mergeCommit"])
	}

	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "pr", "view", "12", "-R", "openclaw/openclaw", "--json", "number,files,commits,statusCheckRollup,headRefOid,headRefName"}); err != nil {
		t.Fatalf("gh pr rich view: %v", err)
	}
	if err := json.Unmarshal(stdout.Bytes(), &view); err != nil {
		t.Fatalf("decode rich view: %v\n%s", err, stdout.String())
	}
	if view["headRefOid"] != "abc123" || len(view["files"].([]any)) != 1 || len(view["commits"].([]any)) != 1 {
		t.Fatalf("rich view = %#v", view)
	}
	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "pr", "view", "12", "-R", "openclaw/openclaw", "--json", "number,isCrossRepository,potentialMergeCommit,autoMergeRequest,reviewRequests"}); err != nil {
		t.Fatalf("gh pr easy fields view: %v", err)
	}
	if err := json.Unmarshal(stdout.Bytes(), &view); err != nil {
		t.Fatalf("decode easy fields view: %v\n%s", err, stdout.String())
	}
	if view["isCrossRepository"] != false || view["autoMergeRequest"] != nil {
		t.Fatalf("easy fields view = %#v", view)
	}
	if view["potentialMergeCommit"].(map[string]any)["oid"] != "merge123" {
		t.Fatalf("easy fields view = %#v", view)
	}
	if got := len(view["reviewRequests"].([]any)); got != 0 {
		t.Fatalf("reviewRequests len = %d, want 0", got)
	}

	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "pr", "checks", "12", "-R", "openclaw/openclaw", "--json", "name,state,detailsUrl,workflow"}); err != nil {
		t.Fatalf("gh pr checks: %v", err)
	}
	var checks []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &checks); err != nil {
		t.Fatalf("decode checks: %v\n%s", err, stdout.String())
	}
	if len(checks) != 1 || checks[0]["name"] != "test" || checks[0]["state"] != "SUCCESS" {
		t.Fatalf("checks = %#v", checks)
	}

	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "pr", "checks", "https://github.com/openclaw/openclaw/pull/12", "--json", "name,state"}); err != nil {
		t.Fatalf("gh pr checks URL: %v", err)
	}
	if err := json.Unmarshal(stdout.Bytes(), &checks); err != nil {
		t.Fatalf("decode URL checks: %v\n%s", err, stdout.String())
	}
	if len(checks) != 1 || checks[0]["name"] != "test" || checks[0]["state"] != "SUCCESS" {
		t.Fatalf("URL checks = %#v", checks)
	}

	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "run", "list", "-R", "openclaw/openclaw", "--branch", "manifest-cache", "--json", "databaseId,workflowName,status,conclusion,headSha"}); err != nil {
		t.Fatalf("gh run list: %v", err)
	}
	var runs []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &runs); err != nil {
		t.Fatalf("decode runs: %v\n%s", err, stdout.String())
	}
	if len(runs) != 1 || int(runs[0]["databaseId"].(float64)) != 99 || runs[0]["headSha"] != "abc123" {
		t.Fatalf("runs = %#v", runs)
	}

	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "run", "view", "99", "-R", "openclaw/openclaw", "--json", "databaseId,url"}); err != nil {
		t.Fatalf("gh run view: %v", err)
	}
	var runView map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &runView); err != nil {
		t.Fatalf("decode run view: %v\n%s", err, stdout.String())
	}
	if int(runView["databaseId"].(float64)) != 99 {
		t.Fatalf("run view = %#v", runView)
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
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "issue", "view", "10", "-R", "openclaw/openclaw", "--json", "number,comments"}); err != nil {
		t.Fatalf("gh issue view comments: %v", err)
	}
	if err := json.Unmarshal(stdout.Bytes(), &view); err != nil {
		t.Fatalf("decode issue comments: %v\n%s", err, stdout.String())
	}
	comments = view["comments"].([]any)
	if len(comments) != 1 || comments[0].(map[string]any)["body"] != "same hot loop here" {
		t.Fatalf("issue comments = %#v", view)
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

	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "issue", "view", "10", "-R", "openclaw/openclaw"}); err != nil {
		t.Fatalf("gh issue human view: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "title:\tHot loop burns CPU") || !strings.Contains(got, "runtime has a hot loop") {
		t.Fatalf("human issue view = %q", got)
	}
	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "issue", "list", "-R", "openclaw/openclaw", "--limit", "1"}); err != nil {
		t.Fatalf("gh issue human list: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "10\tHot loop burns CPU") {
		t.Fatalf("human issue list = %q", got)
	}
	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "pr", "list", "-R", "openclaw/openclaw", "--limit", "1"}); err != nil {
		t.Fatalf("gh pr human list: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "12\tManifest cache update") {
		t.Fatalf("human pr list = %q", got)
	}

	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "--cached", "pr", "list", "-R", "openclaw/openclaw", "--json", "number,headRefName,baseRefName,mergeStateStatus,statusCheckRollup,headRefOid,headRepositoryOwner,maintainerCanModify,reviewDecision"}); err != nil {
		t.Fatalf("gh pr enriched list: %v", err)
	}
	if err := json.Unmarshal(stdout.Bytes(), &list); err != nil {
		t.Fatalf("decode enriched PR list: %v\n%s", err, stdout.String())
	}
	if len(list) != 1 || list[0]["headRefName"] != "manifest-cache" || list[0]["baseRefName"] != "main" || list[0]["mergeStateStatus"] != "CLEAN" || list[0]["headRefOid"] != "abc123" {
		t.Fatalf("enriched PR list = %#v", list)
	}
	if list[0]["headRepositoryOwner"].(map[string]any)["login"] != "openclaw" || list[0]["maintainerCanModify"] != true || len(list[0]["statusCheckRollup"].([]any)) != 1 {
		t.Fatalf("enriched PR list = %#v", list)
	}

	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "api", "repos/openclaw/openclaw/pulls/12", "--jq", "{id, number, title, draft, head: .head.sha, base: .base.ref, headRepoOwner: .head.repo.owner.login, headRepoID: .head.repo.id, userID: .user.id, labelID: .labels[0].id, mergedAt: .merged_at, closedAt: .closed_at, mergeable_state, changed_files}"}); err != nil {
		t.Fatalf("gh api cached pull: %v", err)
	}
	var apiView map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &apiView); err != nil {
		t.Fatalf("decode API view: %v\n%s", err, stdout.String())
	}
	if int(apiView["id"].(float64)) != 12 || int(apiView["number"].(float64)) != 12 || apiView["draft"] != true || apiView["head"] != "abc123" || apiView["base"] != "main" || apiView["headRepoOwner"] != "openclaw" || int(apiView["headRepoID"].(float64)) != 99 || int(apiView["userID"].(float64)) != 42 || int(apiView["labelID"].(float64)) != 7 || apiView["mergeable_state"] != "clean" || int(apiView["changed_files"].(float64)) != 1 {
		t.Fatalf("api view = %#v", apiView)
	}
	if apiView["mergedAt"] != nil || apiView["closedAt"] != nil {
		t.Fatalf("api nullable timestamps = %#v", apiView)
	}
}

func TestParseLocalGHAPIArgsDeclinesBehaviorChangingFlags(t *testing.T) {
	for _, args := range [][]string{
		{"api", "--hostname", "ghe.example.com", "repos/openclaw/openclaw/pulls/12"},
		{"api", "--preview", "mercy", "repos/openclaw/openclaw/pulls/12"},
		{"api", "-H", "Accept: application/vnd.github.raw+json", "repos/openclaw/openclaw/pulls/12"},
		{"api", "--silent", "repos/openclaw/openclaw/pulls/12"},
		{"api", "-i", "repos/openclaw/openclaw/pulls/12"},
		{"api", "--slurp", "repos/openclaw/openclaw/pulls/12"},
		{"api", "repos/openclaw/openclaw/pulls/12", "extra"},
	} {
		if _, _, ok := parseLocalGHAPIArgs(args); ok {
			t.Fatalf("parseLocalGHAPIArgs(%v) ok, want decline", args)
		}
	}
}

func TestGHPRMergedFieldsUseRawPullDetail(t *testing.T) {
	cache := store.PullRequestCache{
		Detail: store.PullRequestDetail{RawJSON: `{"merged":true,"merged_at":"2026-04-27T03:00:00Z","merge_commit_sha":"final123","auto_merge":{"enabled_by":{"login":"alice"},"merge_method":"SQUASH","commit_title":"ship it","commit_message":"details"}}`},
	}
	if !ghPRDetailMerged(store.Thread{}, cache) {
		t.Fatal("merged raw detail should be detected")
	}
	value, err := ghPRDetailJSONValue(store.Thread{}, cache, "potentialMergeCommit")
	if err != nil {
		t.Fatalf("potentialMergeCommit field: %v", err)
	}
	if value != nil {
		t.Fatalf("potentialMergeCommit = %#v, want nil for merged PR", value)
	}
	value, err = ghPRDetailJSONValue(store.Thread{}, cache, "autoMergeRequest")
	if err != nil {
		t.Fatalf("autoMergeRequest field: %v", err)
	}
	autoMerge := value.(map[string]any)
	if autoMerge["mergeMethod"] != "SQUASH" || autoMerge["commitHeadline"] != "ship it" || autoMerge["commitBody"] != "details" || autoMerge["enabledBy"].(map[string]any)["login"] != "alice" {
		t.Fatalf("autoMergeRequest = %#v", autoMerge)
	}
}

func TestGHShimPRChecksWatchFalseUsesLocalCache(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)

	run := New()
	var stdout bytes.Buffer
	run.Stdout = &stdout
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "pr", "checks", "12", "-R", "openclaw/openclaw", "--watch=false", "--json", "name,state"}); err != nil {
		t.Fatalf("gh pr checks --watch=false: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("decode checks: %v\n%s", err, stdout.String())
	}
	if len(rows) != 1 || rows[0]["name"] != "test" {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestGHShimPRListFreshFieldsFallbackWhenDetailStale(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)
	staleGHShimPullRequestCache(t, ctx, configPath)

	dir := t.TempDir()
	ghPath := filepath.Join(dir, "gh")
	if err := os.WriteFile(ghPath, []byte("#!/bin/sh\necho fallback:$*\n"), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("GITCRAWL_GH_PATH", ghPath)
	t.Setenv("GITCRAWL_GH_AUTO_HYDRATE", "0")

	run := New()
	var stdout bytes.Buffer
	run.Stdout = &stdout
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "pr", "list", "-R", "openclaw/openclaw", "--json", "statusCheckRollup"}); err != nil {
		t.Fatalf("gh pr list stale fresh fields: %v", err)
	}
	want := "fallback:pr list -R openclaw/openclaw --json statusCheckRollup"
	if got := strings.TrimSpace(stdout.String()); got != want {
		t.Fatalf("fallback output = %q, want %q", got, want)
	}
}

func TestGHAPILocalPullFallsBackWhenDetailStale(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)
	staleGHShimPullRequestCache(t, ctx, configPath)

	dir := t.TempDir()
	ghPath := filepath.Join(dir, "gh")
	if err := os.WriteFile(ghPath, []byte("#!/bin/sh\nprintf '{\"source\":\"real\"}\\n'\n"), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("GITCRAWL_GH_PATH", ghPath)
	t.Setenv("GITCRAWL_GH_AUTO_HYDRATE", "0")

	run := New()
	var stdout bytes.Buffer
	run.Stdout = &stdout
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "api", "repos/openclaw/openclaw/pulls/12", "--jq", ".source"}); err != nil {
		t.Fatalf("gh api stale pull: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "real" {
		t.Fatalf("api output = %q, want real", got)
	}
}

func TestGHAPILocalPullFallsBackForEnterpriseHost(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)

	dir := t.TempDir()
	ghPath := filepath.Join(dir, "gh")
	if err := os.WriteFile(ghPath, []byte("#!/bin/sh\nprintf '{\"source\":\"enterprise\"}\\n'\n"), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("GITCRAWL_GH_PATH", ghPath)
	t.Setenv("GH_HOST", "ghe.example.com")

	run := New()
	var stdout bytes.Buffer
	run.Stdout = &stdout
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "api", "repos/openclaw/openclaw/pulls/12", "--jq", ".source"}); err != nil {
		t.Fatalf("gh api enterprise host: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "enterprise" {
		t.Fatalf("api output = %q, want enterprise", got)
	}
}

func staleGHShimPullRequestCache(t *testing.T, ctx context.Context, configPath string) {
	t.Helper()
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	repo, err := st.RepositoryByFullName(ctx, "openclaw/openclaw")
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	cache, err := st.PullRequestCache(ctx, repo.ID, 12)
	if err != nil {
		t.Fatalf("pull request cache: %v", err)
	}
	stale := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)
	cache.Detail.FetchedAt = stale
	cache.Detail.UpdatedAt = stale
	if err := st.UpsertPullRequestCache(ctx, cache.Detail, cache.Files, cache.Commits, cache.Checks, nil); err != nil {
		t.Fatalf("stale pull request cache: %v", err)
	}
}

func TestGHReviewDecisionIgnoresStaleReviews(t *testing.T) {
	decision := ghReviewDecision([]map[string]any{
		{"state": "CHANGES_REQUESTED", "isStale": true},
		{"state": "APPROVED", "isStale": false},
	})
	if decision != "APPROVED" {
		t.Fatalf("decision = %#v, want APPROVED", decision)
	}
}

func TestGHCachedReviewFieldsUseLocalRowsOnly(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)
	app := New()
	app.configPath = configPath
	thread, err := app.localGHThread(ctx, "openclaw/openclaw", "pull_request", 12)
	if err != nil {
		t.Fatalf("local thread: %v", err)
	}
	row, err := app.ghThreadViewJSONRow(ctx, "openclaw/openclaw", thread, "reviewDecision,reviews", ghShimControls{Cached: true})
	if err != nil {
		t.Fatalf("cached review row: %v", err)
	}
	if row["reviewDecision"] != nil {
		t.Fatalf("reviewDecision = %#v, want nil", row["reviewDecision"])
	}
	if got := len(row["reviews"].([]map[string]any)); got != 0 {
		t.Fatalf("reviews len = %d, want 0", got)
	}
}

func TestGHReviewDecisionUsesLatestPerReviewer(t *testing.T) {
	decision := ghReviewDecision([]map[string]any{
		{"id": "r2", "author": map[string]any{"login": "alice"}, "state": "APPROVED"},
		{"id": "r1", "author": map[string]any{"login": "alice"}, "state": "CHANGES_REQUESTED"},
	})
	if decision != "APPROVED" {
		t.Fatalf("decision = %#v, want APPROVED", decision)
	}
}

func TestGHReviewsIncludeMoreThanStatusLatestCap(t *testing.T) {
	comments := make([]store.Comment, 0, 12)
	for index := 0; index < 12; index++ {
		comments = append(comments, store.Comment{
			GitHubID:        fmt.Sprintf("r%d", index),
			CommentType:     "pull_review",
			AuthorLogin:     fmt.Sprintf("reviewer-%d", index),
			RawJSON:         fmt.Sprintf(`{"state":"COMMENTED","commit_id":"head","submitted_at":"2026-04-27T%02d:00:00Z"}`, index),
			CreatedAtGitHub: fmt.Sprintf("2026-04-27T%02d:00:00Z", index),
		})
	}
	reviews := ghReviewsJSONValue(comments, "head", false)
	if len(reviews) != 12 {
		t.Fatalf("reviews len = %d, want 12", len(reviews))
	}
}

func TestGHShimUnsupportedChecksAndRunJSONFieldsFallback(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)
	ghPath := filepath.Join(t.TempDir(), "gh")
	if err := os.WriteFile(ghPath, []byte("#!/bin/sh\necho fallback:$*\n"), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("GITCRAWL_GH_PATH", ghPath)

	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "pr checks",
			args: []string{"--config", configPath, "gh", "pr", "checks", "12", "-R", "openclaw/openclaw", "--json", "missingField"},
			want: "fallback:pr checks 12 -R openclaw/openclaw --json missingField",
		},
		{
			name: "run list",
			args: []string{"--config", configPath, "gh", "run", "list", "-R", "openclaw/openclaw", "--branch", "manifest-cache", "--json", "missingField"},
			want: "fallback:run list -R openclaw/openclaw --branch manifest-cache --json missingField",
		},
		{
			name: "run view",
			args: []string{"--config", configPath, "gh", "run", "view", "99", "-R", "openclaw/openclaw", "--json", "missingField"},
			want: "fallback:run view 99 -R openclaw/openclaw --json missingField",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := New()
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			run.Stdout = &stdout
			run.Stderr = &stderr
			if err := run.Run(ctx, tc.args); err != nil {
				t.Fatalf("fallback: %v", err)
			}
			if got := strings.TrimSpace(stdout.String()); got != tc.want {
				t.Fatalf("fallback output = %q, want %q", got, tc.want)
			}
			if got := strings.TrimSpace(stderr.String()); got != "" {
				t.Fatalf("fallback stderr = %q", got)
			}
		})
	}
}

func TestGHShimCachedUnsupportedChecksAndRunJSONFieldsFail(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)
	cases := []struct {
		name string
		args []string
	}{
		{
			name: "pr checks",
			args: []string{"--config", configPath, "gh", "--cached", "pr", "checks", "12", "-R", "openclaw/openclaw", "--json", "missingField"},
		},
		{
			name: "run list",
			args: []string{"--config", configPath, "gh", "--cached", "run", "list", "-R", "openclaw/openclaw", "--branch", "manifest-cache", "--json", "missingField"},
		},
		{
			name: "run view",
			args: []string{"--config", configPath, "gh", "--cached", "run", "view", "99", "-R", "openclaw/openclaw", "--json", "missingField"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := New().Run(ctx, tc.args)
			if err == nil {
				t.Fatal("unsupported cached --json field should fail")
			}
			if !isLocalGHUnsupported(err) || !strings.Contains(err.Error(), "missingField") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestGHMergeableValueMapsNonConflictingStates(t *testing.T) {
	tests := map[string]string{
		"blocked":   "MERGEABLE",
		"behind":    "MERGEABLE",
		"clean":     "MERGEABLE",
		"dirty":     "CONFLICTING",
		"unknown":   "UNKNOWN",
		"unchecked": "UNKNOWN",
	}
	for state, want := range tests {
		if got := ghMergeableValue(state); got != want {
			t.Fatalf("ghMergeableValue(%q) = %q, want %q", state, got, want)
		}
	}
}

func TestGHShimAutoHydratesPRDetailsOnMiss(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	for _, table := range []string{"pull_request_review_thread_syncs", "pull_request_review_threads", "pull_request_checks", "pull_request_commits", "pull_request_files", "pull_request_details", "github_workflow_runs", "threads", "repositories"} {
		if _, err := st.DB().ExecContext(ctx, "delete from "+table); err != nil {
			t.Fatalf("clear %s: %v", table, err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/openclaw/openclaw":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 123, "open_issues_count": 1})
		case "/repos/openclaw/openclaw/issues/12":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 12, "number": 12, "state": "open", "title": "Manifest cache update",
				"body": "", "html_url": "https://github.com/openclaw/openclaw/pull/12",
				"labels": []map[string]any{}, "assignees": []map[string]any{},
				"user":         map[string]any{"login": "bob", "type": "User"},
				"pull_request": map[string]any{"url": "https://api.github.test/repos/openclaw/openclaw/pulls/12"},
			})
		case "/repos/openclaw/openclaw/pulls/12":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number": 12, "head": map[string]any{"sha": "auto123", "ref": "auto-branch", "repo": map[string]any{"full_name": "openclaw/openclaw"}},
				"base": map[string]any{"sha": "base123"}, "mergeable_state": "clean", "changed_files": 1,
			})
		case "/repos/openclaw/openclaw/pulls/12/files":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"filename": "auto.go", "status": "modified"}})
		case "/repos/openclaw/openclaw/pulls/12/commits":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"sha": "commit123", "commit": map[string]any{"message": "test"}}})
		case "/repos/openclaw/openclaw/commits/auto123/check-runs":
			_ = json.NewEncoder(w).Encode(map[string]any{"check_runs": []map[string]any{{"name": "auto-test", "status": "completed", "conclusion": "success"}}})
		case "/repos/openclaw/openclaw/actions/runs":
			_ = json.NewEncoder(w).Encode(map[string]any{"workflow_runs": []map[string]any{{"id": 12345, "head_branch": "auto-branch", "head_sha": "auto123", "status": "completed", "conclusion": "success", "name": "CI"}}})
		case "/graphql":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
				"reviewThreads": map[string]any{"nodes": []map[string]any{}, "pageInfo": map[string]any{"hasNextPage": false, "endCursor": ""}},
			}}}})
		default:
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
	}))
	defer server.Close()
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GITCRAWL_GITHUB_BASE_URL", server.URL)
	t.Setenv("GITCRAWL_GH_PATH", "/tmp/no-real-gh")

	run := New()
	var stdout bytes.Buffer
	run.Stdout = &stdout
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "pr", "view", "12", "-R", "openclaw/openclaw", "--json", "number,files,commits,statusCheckRollup,headRefOid"}); err != nil {
		t.Fatalf("auto hydrate view: %v", err)
	}
	var view map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &view); err != nil {
		t.Fatalf("decode view: %v\n%s", err, stdout.String())
	}
	if view["headRefOid"] != "auto123" || len(view["files"].([]any)) != 1 {
		t.Fatalf("view = %#v", view)
	}
	stdout.Reset()
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "pr", "checks", "12", "-R", "openclaw/openclaw", "--json", "name,state"}); err != nil {
		t.Fatalf("auto hydrate checks: %v", err)
	}
	var checks []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &checks); err != nil {
		t.Fatalf("decode checks: %v\n%s", err, stdout.String())
	}
	if len(checks) != 1 || checks[0]["name"] != "auto-test" || checks[0]["state"] != "SUCCESS" {
		t.Fatalf("checks = %#v", checks)
	}
}
