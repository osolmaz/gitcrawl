package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestUpsertComment(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "gitcrawl.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	repoID, err := st.UpsertRepository(ctx, Repository{Owner: "openclaw", Name: "gitcrawl", FullName: "openclaw/gitcrawl", RawJSON: "{}", UpdatedAt: "2026-04-26T00:00:00Z"})
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	threadID, err := st.UpsertThread(ctx, Thread{
		RepoID: repoID, GitHubID: "1", Number: 1, Kind: "issue", State: "open",
		Title: "download stalls", HTMLURL: "https://github.com/openclaw/gitcrawl/issues/1",
		LabelsJSON: "[]", AssigneesJSON: "[]", RawJSON: "{}", ContentHash: "hash", UpdatedAt: "2026-04-26T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("thread: %v", err)
	}
	id, err := st.UpsertComment(ctx, Comment{
		ThreadID: threadID, GitHubID: "c1", CommentType: "issue_comment",
		AuthorLogin: "vincentkoc", Body: "same bug here", RawJSON: "{}", CreatedAtGitHub: "2026-04-26T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("comment: %v", err)
	}
	if id == 0 {
		t.Fatal("expected comment id")
	}
	if _, err := st.UpsertComment(ctx, Comment{
		ThreadID: threadID, GitHubID: "c0", CommentType: "issue_comment",
		AuthorLogin: "octobot", AuthorType: "Bot", Body: "earlier bot note", IsBot: true, RawJSON: "{}",
		CreatedAtGitHub: "2026-04-25T00:00:00Z", UpdatedAtGitHub: "2026-04-25T00:01:00Z",
	}); err != nil {
		t.Fatalf("second comment: %v", err)
	}
	comments, err := st.ListComments(ctx, threadID)
	if err != nil {
		t.Fatalf("list comments: %v", err)
	}
	if len(comments) != 2 || comments[0].GitHubID != "c0" || !comments[0].IsBot || comments[1].GitHubID != "c1" {
		t.Fatalf("comments = %+v", comments)
	}
}
