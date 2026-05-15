package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertPullRequestReviewThreadsRollsBackReplacementOnInsertError(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "gitcrawl.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	repoID, err := st.UpsertRepository(ctx, Repository{Owner: "openclaw", Name: "gitcrawl", FullName: "openclaw/gitcrawl", RawJSON: "{}", UpdatedAt: "2026-05-15T00:00:00Z"})
	if err != nil {
		t.Fatalf("upsert repo: %v", err)
	}
	threadID, err := st.UpsertThread(ctx, Thread{
		RepoID: repoID, GitHubID: "1", Number: 1, Kind: "pull_request", State: "open",
		Title: "Atomic review threads", HTMLURL: "https://github.com/openclaw/gitcrawl/pull/1",
		LabelsJSON: "[]", AssigneesJSON: "[]", RawJSON: "{}", ContentHash: "hash", UpdatedAt: "2026-05-15T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("upsert thread: %v", err)
	}
	oldFetchedAt := "2026-05-15T00:00:00Z"
	if err := st.UpsertPullRequestReviewThreads(ctx, threadID, oldFetchedAt, []PullRequestReviewThread{{
		ReviewThreadID: "old", IsResolved: false, CommentsJSON: "[]", RawJSON: "{}", FetchedAt: oldFetchedAt,
	}}); err != nil {
		t.Fatalf("seed review threads: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx, `
		create trigger fail_review_thread_insert
		before insert on pull_request_review_threads
		begin
			select raise(fail, 'blocked review thread insert');
		end;
	`); err != nil {
		t.Fatalf("create fail trigger: %v", err)
	}
	defer st.DB().ExecContext(ctx, `drop trigger if exists fail_review_thread_insert`)

	err = st.UpsertPullRequestReviewThreads(ctx, threadID, "2026-05-15T00:01:00Z", []PullRequestReviewThread{{
		ReviewThreadID: "new", IsResolved: true, CommentsJSON: "[]", RawJSON: "{}", FetchedAt: "2026-05-15T00:01:00Z",
	}})
	if err == nil || !strings.Contains(err.Error(), "blocked review thread insert") {
		t.Fatalf("expected trigger error, got %v", err)
	}
	threads, err := st.PullRequestReviewThreads(ctx, threadID)
	if err != nil {
		t.Fatalf("list review threads: %v", err)
	}
	if len(threads) != 1 || threads[0].ReviewThreadID != "old" {
		t.Fatalf("review thread replacement should roll back, got %+v", threads)
	}
	fetchedAt, err := st.PullRequestReviewThreadsFetchedAt(ctx, threadID)
	if err != nil {
		t.Fatalf("review thread fetched marker: %v", err)
	}
	if fetchedAt != oldFetchedAt {
		t.Fatalf("fetched marker = %q, want %q", fetchedAt, oldFetchedAt)
	}
}
