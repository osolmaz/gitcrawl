package syncer

import (
	"context"
	"time"

	gh "github.com/openclaw/gitcrawl/internal/github"
	"github.com/openclaw/gitcrawl/internal/store"
)

type pullDetailStats struct {
	files   int
	commits int
	checks  int
	runs    int
}

func (s *Syncer) syncPullRequestDetails(ctx context.Context, st *store.Store, options Options, thread store.Thread) (pullDetailStats, error) {
	fetchedAt := s.now().Format(time.RFC3339Nano)
	pull, err := s.client.GetPull(ctx, options.Owner, options.Repo, thread.Number, options.Reporter)
	if err != nil {
		return pullDetailStats{}, err
	}
	filesRaw, err := s.client.ListPullFiles(ctx, options.Owner, options.Repo, thread.Number, options.Reporter)
	if err != nil {
		return pullDetailStats{}, err
	}
	commitsRaw, err := s.client.ListPullCommits(ctx, options.Owner, options.Repo, thread.Number, options.Reporter)
	if err != nil {
		return pullDetailStats{}, err
	}
	headSHA := nestedString(pull, "head", "sha")
	var checksRaw []map[string]any
	if headSHA != "" {
		checksRaw, err = s.client.ListCommitCheckRuns(ctx, options.Owner, options.Repo, headSHA, options.Reporter)
		if err != nil {
			return pullDetailStats{}, err
		}
	}
	var runsRaw []map[string]any
	if headSHA != "" {
		runsRaw, err = s.client.ListWorkflowRuns(ctx, options.Owner, options.Repo, gh.ListWorkflowRunsOptions{HeadSHA: headSHA, Limit: 20}, options.Reporter)
		if err != nil {
			return pullDetailStats{}, err
		}
	}
	detail := mapPullDetail(thread, pull, fetchedAt)
	files := mapPullFiles(thread.ID, filesRaw, fetchedAt)
	commits := mapPullCommits(thread.ID, commitsRaw, fetchedAt)
	checks := mapPullChecks(thread.ID, checksRaw, fetchedAt)
	runs := mapWorkflowRuns(thread.RepoID, runsRaw, fetchedAt)
	if err := st.UpsertPullRequestCache(ctx, detail, files, commits, checks, runs); err != nil {
		return pullDetailStats{}, err
	}
	return pullDetailStats{files: len(files), commits: len(commits), checks: len(checks), runs: len(runs)}, nil
}

func mapPullDetail(thread store.Thread, pull map[string]any, fetchedAt string) store.PullRequestDetail {
	return store.PullRequestDetail{
		ThreadID:         thread.ID,
		RepoID:           thread.RepoID,
		Number:           thread.Number,
		BaseSHA:          nestedString(pull, "base", "sha"),
		HeadSHA:          nestedString(pull, "head", "sha"),
		HeadRef:          nestedString(pull, "head", "ref"),
		HeadRepoFullName: nestedString(pull, "head", "repo", "full_name"),
		MergeableState:   stringValue(pull["mergeable_state"]),
		Additions:        intValue(pull["additions"]),
		Deletions:        intValue(pull["deletions"]),
		ChangedFiles:     intValue(pull["changed_files"]),
		RawJSON:          mustJSON(pull),
		FetchedAt:        fetchedAt,
		UpdatedAt:        fetchedAt,
	}
}

func mapPullFiles(threadID int64, rows []map[string]any, fetchedAt string) []store.PullRequestFile {
	out := make([]store.PullRequestFile, 0, len(rows))
	for _, row := range rows {
		filename := stringValue(row["filename"])
		if filename == "" {
			continue
		}
		out = append(out, store.PullRequestFile{
			ThreadID:     threadID,
			Path:         filename,
			Status:       stringValue(row["status"]),
			Additions:    intValue(row["additions"]),
			Deletions:    intValue(row["deletions"]),
			Changes:      intValue(row["changes"]),
			PreviousPath: stringValue(row["previous_filename"]),
			Patch:        stringValue(row["patch"]),
			RawJSON:      mustJSON(row),
			FetchedAt:    fetchedAt,
		})
	}
	return out
}

func mapPullCommits(threadID int64, rows []map[string]any, fetchedAt string) []store.PullRequestCommit {
	out := make([]store.PullRequestCommit, 0, len(rows))
	for _, row := range rows {
		sha := stringValue(row["sha"])
		if sha == "" {
			continue
		}
		out = append(out, store.PullRequestCommit{
			ThreadID:    threadID,
			SHA:         sha,
			Message:     nestedString(row, "commit", "message"),
			AuthorLogin: nestedString(row, "author", "login"),
			AuthorName:  nestedString(row, "commit", "author", "name"),
			CommittedAt: nestedString(row, "commit", "author", "date"),
			HTMLURL:     stringValue(row["html_url"]),
			RawJSON:     mustJSON(row),
			FetchedAt:   fetchedAt,
		})
	}
	return out
}
