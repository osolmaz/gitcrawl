package cli

import (
	"context"
	"testing"

	"github.com/openclaw/gitcrawl/internal/store"
)

func prIDForTest(t *testing.T, ctx context.Context, st *store.Store, repoID int64, number int) int64 {
	t.Helper()
	threads, err := st.ListThreadsFiltered(ctx, store.ThreadListOptions{
		RepoID:        repoID,
		IncludeClosed: true,
		Numbers:       []int{number},
	})
	if err != nil {
		t.Fatalf("list PR for test: %v", err)
	}
	for _, thread := range threads {
		if thread.Number == number && thread.Kind == "pull_request" {
			return thread.ID
		}
	}
	t.Fatalf("missing PR #%d", number)
	return 0
}
