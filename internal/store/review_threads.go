package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/openclaw/gitcrawl/internal/store/storedb"
)

type PullRequestReviewThread struct {
	ThreadID              int64  `json:"thread_id"`
	ReviewThreadID        string `json:"id"`
	Path                  string `json:"path,omitempty"`
	Line                  int    `json:"line,omitempty"`
	StartLine             int    `json:"start_line,omitempty"`
	IsResolved            bool   `json:"is_resolved"`
	IsOutdated            bool   `json:"is_outdated"`
	ViewerCanResolve      bool   `json:"viewer_can_resolve"`
	ViewerCanUnresolve    bool   `json:"viewer_can_unresolve"`
	ViewerCanReply        bool   `json:"viewer_can_reply"`
	FirstAuthorLogin      string `json:"first_author_login,omitempty"`
	FirstAuthorType       string `json:"first_author_type,omitempty"`
	FirstCommentBody      string `json:"first_comment_body,omitempty"`
	FirstCommentURL       string `json:"first_comment_url,omitempty"`
	FirstCommentCreatedAt string `json:"first_comment_created_at,omitempty"`
	FirstCommentUpdatedAt string `json:"first_comment_updated_at,omitempty"`
	CommentsJSON          string `json:"comments_json"`
	RawJSON               string `json:"-"`
	FetchedAt             string `json:"fetched_at"`
}

func (s *Store) UpsertPullRequestReviewThreads(ctx context.Context, threadID int64, fetchedAt string, threads []PullRequestReviewThread) error {
	if s.queries == nil {
		return s.WithTx(ctx, func(tx *Store) error {
			return tx.upsertPullRequestReviewThreads(ctx, threadID, fetchedAt, threads)
		})
	}
	return s.upsertPullRequestReviewThreads(ctx, threadID, fetchedAt, threads)
}

func (s *Store) upsertPullRequestReviewThreads(ctx context.Context, threadID int64, fetchedAt string, threads []PullRequestReviewThread) error {
	if err := s.qsql().DeletePullRequestReviewThreads(ctx, threadID); err != nil {
		return fmt.Errorf("clear pull request review threads: %w", err)
	}
	if err := s.qsql().UpsertPullRequestReviewThreadSync(ctx, storedb.UpsertPullRequestReviewThreadSyncParams{
		ThreadID:  threadID,
		FetchedAt: fetchedAt,
	}); err != nil {
		return fmt.Errorf("mark pull request review threads fetched: %w", err)
	}
	for _, thread := range threads {
		if thread.ReviewThreadID == "" {
			continue
		}
		if err := s.qsql().UpsertPullRequestReviewThread(ctx, storedb.UpsertPullRequestReviewThreadParams{
			ThreadID:              threadID,
			ReviewThreadID:        thread.ReviewThreadID,
			Path:                  nullString(thread.Path),
			Line:                  int64(thread.Line),
			StartLine:             int64(thread.StartLine),
			IsResolved:            int64(boolInt(thread.IsResolved)),
			IsOutdated:            int64(boolInt(thread.IsOutdated)),
			ViewerCanResolve:      int64(boolInt(thread.ViewerCanResolve)),
			ViewerCanUnresolve:    int64(boolInt(thread.ViewerCanUnresolve)),
			ViewerCanReply:        int64(boolInt(thread.ViewerCanReply)),
			FirstAuthorLogin:      nullString(thread.FirstAuthorLogin),
			FirstAuthorType:       nullString(thread.FirstAuthorType),
			FirstCommentBody:      nullString(thread.FirstCommentBody),
			FirstCommentUrl:       nullString(thread.FirstCommentURL),
			FirstCommentCreatedAt: nullString(thread.FirstCommentCreatedAt),
			FirstCommentUpdatedAt: nullString(thread.FirstCommentUpdatedAt),
			CommentsJson:          thread.CommentsJSON,
			RawJson:               thread.RawJSON,
			FetchedAt:             thread.FetchedAt,
		}); err != nil {
			return fmt.Errorf("upsert pull request review thread: %w", err)
		}
	}
	return nil
}

func (s *Store) PullRequestReviewThreads(ctx context.Context, threadID int64) ([]PullRequestReviewThread, error) {
	if !s.tableExists(ctx, "pull_request_review_threads") {
		return nil, nil
	}
	rows, err := s.qsql().PullRequestReviewThreads(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("list pull request review threads: %w", err)
	}
	threads := make([]PullRequestReviewThread, 0, len(rows))
	for _, row := range rows {
		threads = append(threads, PullRequestReviewThread{
			ThreadID:              row.ThreadID,
			ReviewThreadID:        row.ReviewThreadID,
			Path:                  stringValue(row.Path),
			Line:                  int(row.Line),
			StartLine:             int(row.StartLine),
			IsResolved:            int64Bool(row.IsResolved),
			IsOutdated:            int64Bool(row.IsOutdated),
			ViewerCanResolve:      int64Bool(row.ViewerCanResolve),
			ViewerCanUnresolve:    int64Bool(row.ViewerCanUnresolve),
			ViewerCanReply:        int64Bool(row.ViewerCanReply),
			FirstAuthorLogin:      stringValue(row.FirstAuthorLogin),
			FirstAuthorType:       stringValue(row.FirstAuthorType),
			FirstCommentBody:      stringValue(row.FirstCommentBody),
			FirstCommentURL:       stringValue(row.FirstCommentUrl),
			FirstCommentCreatedAt: stringValue(row.FirstCommentCreatedAt),
			FirstCommentUpdatedAt: stringValue(row.FirstCommentUpdatedAt),
			CommentsJSON:          row.CommentsJson,
			RawJSON:               row.RawJson,
			FetchedAt:             row.FetchedAt,
		})
	}
	return threads, nil
}

func (s *Store) PullRequestReviewThreadsFetchedAt(ctx context.Context, threadID int64) (string, error) {
	if !s.tableExists(ctx, "pull_request_review_thread_syncs") {
		return "", nil
	}
	fetchedAt, err := s.qsql().PullRequestReviewThreadsFetchedAt(ctx, threadID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("pull request review threads fetched marker: %w", err)
	}
	return fetchedAt, nil
}
