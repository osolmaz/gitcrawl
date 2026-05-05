package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type PullRequestDetail struct {
	ThreadID         int64  `json:"thread_id"`
	RepoID           int64  `json:"repo_id"`
	Number           int    `json:"number"`
	BaseSHA          string `json:"base_sha,omitempty"`
	HeadSHA          string `json:"head_sha,omitempty"`
	HeadRef          string `json:"head_ref,omitempty"`
	HeadRepoFullName string `json:"head_repo_full_name,omitempty"`
	MergeableState   string `json:"mergeable_state,omitempty"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	ChangedFiles     int    `json:"changed_files"`
	RawJSON          string `json:"raw_json,omitempty"`
	FetchedAt        string `json:"fetched_at"`
	UpdatedAt        string `json:"updated_at"`
}

type PullRequestFile struct {
	ThreadID     int64  `json:"thread_id"`
	Path         string `json:"path"`
	Status       string `json:"status,omitempty"`
	Additions    int    `json:"additions"`
	Deletions    int    `json:"deletions"`
	Changes      int    `json:"changes"`
	PreviousPath string `json:"previous_path,omitempty"`
	Patch        string `json:"patch,omitempty"`
	RawJSON      string `json:"raw_json,omitempty"`
	FetchedAt    string `json:"fetched_at"`
}

type PullRequestCommit struct {
	ThreadID    int64  `json:"thread_id"`
	SHA         string `json:"sha"`
	Message     string `json:"message,omitempty"`
	AuthorLogin string `json:"author_login,omitempty"`
	AuthorName  string `json:"author_name,omitempty"`
	CommittedAt string `json:"committed_at,omitempty"`
	HTMLURL     string `json:"html_url,omitempty"`
	RawJSON     string `json:"raw_json,omitempty"`
	FetchedAt   string `json:"fetched_at"`
}

type PullRequestCheck struct {
	ID           int64  `json:"id"`
	ThreadID     int64  `json:"thread_id"`
	Name         string `json:"name"`
	Status       string `json:"status,omitempty"`
	Conclusion   string `json:"conclusion,omitempty"`
	DetailsURL   string `json:"details_url,omitempty"`
	WorkflowName string `json:"workflow_name,omitempty"`
	StartedAt    string `json:"started_at,omitempty"`
	CompletedAt  string `json:"completed_at,omitempty"`
	RawJSON      string `json:"raw_json,omitempty"`
	FetchedAt    string `json:"fetched_at"`
}

type WorkflowRun struct {
	RepoID       int64  `json:"repo_id"`
	RunID        string `json:"run_id"`
	RunNumber    int    `json:"run_number"`
	HeadBranch   string `json:"head_branch,omitempty"`
	HeadSHA      string `json:"head_sha,omitempty"`
	Status       string `json:"status,omitempty"`
	Conclusion   string `json:"conclusion,omitempty"`
	WorkflowName string `json:"workflow_name,omitempty"`
	Event        string `json:"event,omitempty"`
	HTMLURL      string `json:"html_url,omitempty"`
	CreatedAtGH  string `json:"created_at_gh,omitempty"`
	UpdatedAtGH  string `json:"updated_at_gh,omitempty"`
	RawJSON      string `json:"raw_json,omitempty"`
	FetchedAt    string `json:"fetched_at"`
}

type PullRequestCache struct {
	Detail  PullRequestDetail   `json:"detail"`
	Files   []PullRequestFile   `json:"files"`
	Commits []PullRequestCommit `json:"commits"`
	Checks  []PullRequestCheck  `json:"checks"`
}

func (s *Store) UpsertPullRequestCache(ctx context.Context, detail PullRequestDetail, files []PullRequestFile, commits []PullRequestCommit, checks []PullRequestCheck, runs []WorkflowRun) error {
	if s.queries != nil {
		return s.upsertPullRequestCache(ctx, detail, files, commits, checks, runs)
	}
	return s.WithTx(ctx, func(tx *Store) error {
		return tx.upsertPullRequestCache(ctx, detail, files, commits, checks, runs)
	})
}

func (s *Store) upsertPullRequestCache(ctx context.Context, detail PullRequestDetail, files []PullRequestFile, commits []PullRequestCommit, checks []PullRequestCheck, runs []WorkflowRun) error {
	if _, err := s.q().ExecContext(ctx, `
			insert into pull_request_details(thread_id, repo_id, number, base_sha, head_sha, head_ref, head_repo_full_name, mergeable_state, additions, deletions, changed_files, raw_json, fetched_at, updated_at)
			values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			on conflict(thread_id) do update set
				repo_id=excluded.repo_id,
				number=excluded.number,
				base_sha=excluded.base_sha,
				head_sha=excluded.head_sha,
				head_ref=excluded.head_ref,
				head_repo_full_name=excluded.head_repo_full_name,
				mergeable_state=excluded.mergeable_state,
				additions=excluded.additions,
				deletions=excluded.deletions,
				changed_files=excluded.changed_files,
				raw_json=excluded.raw_json,
				fetched_at=excluded.fetched_at,
				updated_at=excluded.updated_at
		`, detail.ThreadID, detail.RepoID, detail.Number, nullString(detail.BaseSHA), nullString(detail.HeadSHA), nullString(detail.HeadRef), nullString(detail.HeadRepoFullName), nullString(detail.MergeableState), detail.Additions, detail.Deletions, detail.ChangedFiles, detail.RawJSON, detail.FetchedAt, detail.UpdatedAt); err != nil {
		return fmt.Errorf("upsert pull request detail: %w", err)
	}
	if _, err := s.q().ExecContext(ctx, `delete from pull_request_files where thread_id = ?`, detail.ThreadID); err != nil {
		return fmt.Errorf("clear pull request files: %w", err)
	}
	for _, file := range files {
		if _, err := s.q().ExecContext(ctx, `
				insert into pull_request_files(thread_id, path, status, additions, deletions, changes, previous_path, patch, raw_json, fetched_at)
				values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, detail.ThreadID, file.Path, nullString(file.Status), file.Additions, file.Deletions, file.Changes, nullString(file.PreviousPath), nullString(file.Patch), file.RawJSON, file.FetchedAt); err != nil {
			return fmt.Errorf("upsert pull request file: %w", err)
		}
	}
	if _, err := s.q().ExecContext(ctx, `delete from pull_request_commits where thread_id = ?`, detail.ThreadID); err != nil {
		return fmt.Errorf("clear pull request commits: %w", err)
	}
	for _, commit := range commits {
		if _, err := s.q().ExecContext(ctx, `
				insert into pull_request_commits(thread_id, sha, message, author_login, author_name, committed_at, html_url, raw_json, fetched_at)
				values(?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, detail.ThreadID, commit.SHA, nullString(commit.Message), nullString(commit.AuthorLogin), nullString(commit.AuthorName), nullString(commit.CommittedAt), nullString(commit.HTMLURL), commit.RawJSON, commit.FetchedAt); err != nil {
			return fmt.Errorf("upsert pull request commit: %w", err)
		}
	}
	if _, err := s.q().ExecContext(ctx, `delete from pull_request_checks where thread_id = ?`, detail.ThreadID); err != nil {
		return fmt.Errorf("clear pull request checks: %w", err)
	}
	for _, check := range checks {
		if _, err := s.q().ExecContext(ctx, `
				insert into pull_request_checks(thread_id, name, status, conclusion, details_url, workflow_name, started_at, completed_at, raw_json, fetched_at)
				values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, detail.ThreadID, check.Name, nullString(check.Status), nullString(check.Conclusion), nullString(check.DetailsURL), nullString(check.WorkflowName), nullString(check.StartedAt), nullString(check.CompletedAt), check.RawJSON, check.FetchedAt); err != nil {
			return fmt.Errorf("upsert pull request check: %w", err)
		}
	}
	for _, run := range runs {
		if _, err := s.q().ExecContext(ctx, `
				insert into github_workflow_runs(repo_id, run_id, run_number, head_branch, head_sha, status, conclusion, workflow_name, event, html_url, created_at_gh, updated_at_gh, raw_json, fetched_at)
				values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				on conflict(repo_id, run_id) do update set
					run_number=excluded.run_number,
					head_branch=excluded.head_branch,
					head_sha=excluded.head_sha,
					status=excluded.status,
					conclusion=excluded.conclusion,
					workflow_name=excluded.workflow_name,
					event=excluded.event,
					html_url=excluded.html_url,
					created_at_gh=excluded.created_at_gh,
					updated_at_gh=excluded.updated_at_gh,
					raw_json=excluded.raw_json,
					fetched_at=excluded.fetched_at
			`, run.RepoID, run.RunID, run.RunNumber, nullString(run.HeadBranch), nullString(run.HeadSHA), nullString(run.Status), nullString(run.Conclusion), nullString(run.WorkflowName), nullString(run.Event), nullString(run.HTMLURL), nullString(run.CreatedAtGH), nullString(run.UpdatedAtGH), run.RawJSON, run.FetchedAt); err != nil {
			return fmt.Errorf("upsert workflow run: %w", err)
		}
	}
	return nil
}

func (s *Store) PullRequestCache(ctx context.Context, repoID int64, number int) (PullRequestCache, error) {
	var cache PullRequestCache
	var baseSHA, headSHA, headRef, headRepo, mergeable sql.NullString
	err := s.q().QueryRowContext(ctx, `
		select thread_id, repo_id, number, base_sha, head_sha, head_ref, head_repo_full_name, mergeable_state, additions, deletions, changed_files, raw_json, fetched_at, updated_at
		from pull_request_details
		where repo_id = ? and number = ?
	`, repoID, number).Scan(&cache.Detail.ThreadID, &cache.Detail.RepoID, &cache.Detail.Number, &baseSHA, &headSHA, &headRef, &headRepo, &mergeable, &cache.Detail.Additions, &cache.Detail.Deletions, &cache.Detail.ChangedFiles, &cache.Detail.RawJSON, &cache.Detail.FetchedAt, &cache.Detail.UpdatedAt)
	if err != nil {
		return PullRequestCache{}, fmt.Errorf("pull request detail: %w", err)
	}
	cache.Detail.BaseSHA = baseSHA.String
	cache.Detail.HeadSHA = headSHA.String
	cache.Detail.HeadRef = headRef.String
	cache.Detail.HeadRepoFullName = headRepo.String
	cache.Detail.MergeableState = mergeable.String
	files, err := s.PullRequestFiles(ctx, cache.Detail.ThreadID)
	if err != nil {
		return PullRequestCache{}, err
	}
	cache.Files = files
	commits, err := s.PullRequestCommits(ctx, cache.Detail.ThreadID)
	if err != nil {
		return PullRequestCache{}, err
	}
	cache.Commits = commits
	checks, err := s.PullRequestChecks(ctx, cache.Detail.ThreadID)
	if err != nil {
		return PullRequestCache{}, err
	}
	cache.Checks = checks
	return cache, nil
}

func (s *Store) PullRequestFiles(ctx context.Context, threadID int64) ([]PullRequestFile, error) {
	rows, err := s.q().QueryContext(ctx, `
		select thread_id, path, status, additions, deletions, changes, previous_path, patch, raw_json, fetched_at
		from pull_request_files
		where thread_id = ?
		order by path
	`, threadID)
	if err != nil {
		return nil, fmt.Errorf("list pull request files: %w", err)
	}
	defer rows.Close()
	var out []PullRequestFile
	for rows.Next() {
		var file PullRequestFile
		var status, previousPath, patch sql.NullString
		if err := rows.Scan(&file.ThreadID, &file.Path, &status, &file.Additions, &file.Deletions, &file.Changes, &previousPath, &patch, &file.RawJSON, &file.FetchedAt); err != nil {
			return nil, fmt.Errorf("scan pull request file: %w", err)
		}
		file.Status = status.String
		file.PreviousPath = previousPath.String
		file.Patch = patch.String
		out = append(out, file)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pull request files: %w", err)
	}
	return out, nil
}

func (s *Store) PullRequestCommits(ctx context.Context, threadID int64) ([]PullRequestCommit, error) {
	rows, err := s.q().QueryContext(ctx, `
		select thread_id, sha, message, author_login, author_name, committed_at, html_url, raw_json, fetched_at
		from pull_request_commits
		where thread_id = ?
		order by rowid
	`, threadID)
	if err != nil {
		return nil, fmt.Errorf("list pull request commits: %w", err)
	}
	defer rows.Close()
	var out []PullRequestCommit
	for rows.Next() {
		var commit PullRequestCommit
		var message, authorLogin, authorName, committedAt, htmlURL sql.NullString
		if err := rows.Scan(&commit.ThreadID, &commit.SHA, &message, &authorLogin, &authorName, &committedAt, &htmlURL, &commit.RawJSON, &commit.FetchedAt); err != nil {
			return nil, fmt.Errorf("scan pull request commit: %w", err)
		}
		commit.Message = message.String
		commit.AuthorLogin = authorLogin.String
		commit.AuthorName = authorName.String
		commit.CommittedAt = committedAt.String
		commit.HTMLURL = htmlURL.String
		out = append(out, commit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pull request commits: %w", err)
	}
	return out, nil
}

func (s *Store) PullRequestChecks(ctx context.Context, threadID int64) ([]PullRequestCheck, error) {
	rows, err := s.q().QueryContext(ctx, `
		select id, thread_id, name, status, conclusion, details_url, workflow_name, started_at, completed_at, raw_json, fetched_at
		from pull_request_checks
		where thread_id = ?
		order by name
	`, threadID)
	if err != nil {
		return nil, fmt.Errorf("list pull request checks: %w", err)
	}
	defer rows.Close()
	var out []PullRequestCheck
	for rows.Next() {
		var check PullRequestCheck
		var status, conclusion, detailsURL, workflowName, startedAt, completedAt sql.NullString
		if err := rows.Scan(&check.ID, &check.ThreadID, &check.Name, &status, &conclusion, &detailsURL, &workflowName, &startedAt, &completedAt, &check.RawJSON, &check.FetchedAt); err != nil {
			return nil, fmt.Errorf("scan pull request check: %w", err)
		}
		check.Status = status.String
		check.Conclusion = conclusion.String
		check.DetailsURL = detailsURL.String
		check.WorkflowName = workflowName.String
		check.StartedAt = startedAt.String
		check.CompletedAt = completedAt.String
		out = append(out, check)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pull request checks: %w", err)
	}
	return out, nil
}

type WorkflowRunListOptions struct {
	Branch  string
	HeadSHA string
	Limit   int
}

func (s *Store) ListWorkflowRuns(ctx context.Context, repoID int64, options WorkflowRunListOptions) ([]WorkflowRun, error) {
	where := []string{"repo_id = ?"}
	args := []any{repoID}
	if options.Branch != "" {
		where = append(where, "head_branch = ?")
		args = append(args, options.Branch)
	}
	if options.HeadSHA != "" {
		where = append(where, "head_sha = ?")
		args = append(args, options.HeadSHA)
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit)
	rows, err := s.q().QueryContext(ctx, `
		select repo_id, run_id, run_number, head_branch, head_sha, status, conclusion, workflow_name, event, html_url, created_at_gh, updated_at_gh, raw_json, fetched_at
		from github_workflow_runs
		where `+strings.Join(where, " and ")+`
		order by updated_at_gh desc, run_id desc
		limit ?
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("list workflow runs: %w", err)
	}
	defer rows.Close()
	var out []WorkflowRun
	for rows.Next() {
		var run WorkflowRun
		var branch, sha, status, conclusion, workflowName, event, htmlURL, createdAt, updatedAt sql.NullString
		if err := rows.Scan(&run.RepoID, &run.RunID, &run.RunNumber, &branch, &sha, &status, &conclusion, &workflowName, &event, &htmlURL, &createdAt, &updatedAt, &run.RawJSON, &run.FetchedAt); err != nil {
			return nil, fmt.Errorf("scan workflow run: %w", err)
		}
		run.HeadBranch = branch.String
		run.HeadSHA = sha.String
		run.Status = status.String
		run.Conclusion = conclusion.String
		run.WorkflowName = workflowName.String
		run.Event = event.String
		run.HTMLURL = htmlURL.String
		run.CreatedAtGH = createdAt.String
		run.UpdatedAtGH = updatedAt.String
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow runs: %w", err)
	}
	return out, nil
}
