package store

import (
	"context"
	"database/sql"
	"fmt"
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
	if _, err := s.q().ExecContext(ctx, `delete from pull_request_review_threads where thread_id = ?`, threadID); err != nil {
		return fmt.Errorf("clear pull request review threads: %w", err)
	}
	if _, err := s.q().ExecContext(ctx, `
		insert into pull_request_review_thread_syncs(thread_id, fetched_at)
		values(?, ?)
		on conflict(thread_id) do update set fetched_at=excluded.fetched_at
	`, threadID, fetchedAt); err != nil {
		return fmt.Errorf("mark pull request review threads fetched: %w", err)
	}
	for _, thread := range threads {
		if thread.ReviewThreadID == "" {
			continue
		}
		if _, err := s.q().ExecContext(ctx, `
			insert into pull_request_review_threads(
				thread_id, review_thread_id, path, line, start_line, is_resolved, is_outdated,
				viewer_can_resolve, viewer_can_unresolve, viewer_can_reply,
				first_author_login, first_author_type, first_comment_body, first_comment_url,
				first_comment_created_at, first_comment_updated_at, comments_json, raw_json, fetched_at
			)
			values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			on conflict(thread_id, review_thread_id) do update set
				path=excluded.path,
				line=excluded.line,
				start_line=excluded.start_line,
				is_resolved=excluded.is_resolved,
				is_outdated=excluded.is_outdated,
				viewer_can_resolve=excluded.viewer_can_resolve,
				viewer_can_unresolve=excluded.viewer_can_unresolve,
				viewer_can_reply=excluded.viewer_can_reply,
				first_author_login=excluded.first_author_login,
				first_author_type=excluded.first_author_type,
				first_comment_body=excluded.first_comment_body,
				first_comment_url=excluded.first_comment_url,
				first_comment_created_at=excluded.first_comment_created_at,
				first_comment_updated_at=excluded.first_comment_updated_at,
				comments_json=excluded.comments_json,
				raw_json=excluded.raw_json,
				fetched_at=excluded.fetched_at
		`, threadID, thread.ReviewThreadID, nullString(thread.Path), thread.Line, thread.StartLine, boolInt(thread.IsResolved), boolInt(thread.IsOutdated),
			boolInt(thread.ViewerCanResolve), boolInt(thread.ViewerCanUnresolve), boolInt(thread.ViewerCanReply),
			nullString(thread.FirstAuthorLogin), nullString(thread.FirstAuthorType), nullString(thread.FirstCommentBody), nullString(thread.FirstCommentURL),
			nullString(thread.FirstCommentCreatedAt), nullString(thread.FirstCommentUpdatedAt), thread.CommentsJSON, thread.RawJSON, thread.FetchedAt); err != nil {
			return fmt.Errorf("upsert pull request review thread: %w", err)
		}
	}
	return nil
}

func (s *Store) PullRequestReviewThreads(ctx context.Context, threadID int64) ([]PullRequestReviewThread, error) {
	if !s.tableExists(ctx, "pull_request_review_threads") {
		return nil, nil
	}
	rows, err := s.q().QueryContext(ctx, `
		select thread_id, review_thread_id, path, line, start_line, is_resolved, is_outdated,
			viewer_can_resolve, viewer_can_unresolve, viewer_can_reply,
			first_author_login, first_author_type, first_comment_body, first_comment_url,
			first_comment_created_at, first_comment_updated_at, comments_json, raw_json, fetched_at
		from pull_request_review_threads
		where thread_id = ?
		order by is_resolved, path, line, review_thread_id
	`, threadID)
	if err != nil {
		return nil, fmt.Errorf("list pull request review threads: %w", err)
	}
	defer rows.Close()
	var threads []PullRequestReviewThread
	for rows.Next() {
		var thread PullRequestReviewThread
		var path, firstAuthor, firstAuthorType, firstBody, firstURL, firstCreated, firstUpdated sql.NullString
		var resolved, outdated, canResolve, canUnresolve, canReply int
		if err := rows.Scan(&thread.ThreadID, &thread.ReviewThreadID, &path, &thread.Line, &thread.StartLine, &resolved, &outdated,
			&canResolve, &canUnresolve, &canReply, &firstAuthor, &firstAuthorType, &firstBody, &firstURL,
			&firstCreated, &firstUpdated, &thread.CommentsJSON, &thread.RawJSON, &thread.FetchedAt); err != nil {
			return nil, fmt.Errorf("scan pull request review thread: %w", err)
		}
		thread.Path = path.String
		thread.IsResolved = resolved != 0
		thread.IsOutdated = outdated != 0
		thread.ViewerCanResolve = canResolve != 0
		thread.ViewerCanUnresolve = canUnresolve != 0
		thread.ViewerCanReply = canReply != 0
		thread.FirstAuthorLogin = firstAuthor.String
		thread.FirstAuthorType = firstAuthorType.String
		thread.FirstCommentBody = firstBody.String
		thread.FirstCommentURL = firstURL.String
		thread.FirstCommentCreatedAt = firstCreated.String
		thread.FirstCommentUpdatedAt = firstUpdated.String
		threads = append(threads, thread)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pull request review threads: %w", err)
	}
	return threads, nil
}

func (s *Store) PullRequestReviewThreadsFetchedAt(ctx context.Context, threadID int64) (string, error) {
	if !s.tableExists(ctx, "pull_request_review_thread_syncs") {
		return "", nil
	}
	var fetchedAt sql.NullString
	err := s.q().QueryRowContext(ctx, `select fetched_at from pull_request_review_thread_syncs where thread_id = ?`, threadID).Scan(&fetchedAt)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("pull request review threads fetched marker: %w", err)
	}
	return fetchedAt.String, nil
}
