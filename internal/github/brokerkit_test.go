package github

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
)

func TestBrokerKitGetIssueUsesTypedOperation(t *testing.T) {
	t.Parallel()
	var got []string
	client := NewBrokerKit(BrokerKitOptions{Runner: func(_ context.Context, args ...string) ([]byte, error) {
		got = slices.Clone(args)
		return []byte(`{"state":"succeeded","result":{"id":7,"number":42,"title":"Local model","body":"Ollama","html_url":"https://github.com/openclaw/openclaw/issues/42","user":{"login":"alice","type":"User"},"labels":[{"name":"bug"}],"assignees":[],"state":"open","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-02T00:00:00Z"}}`), nil
	}})
	row, err := client.GetIssue(context.Background(), "openclaw", "openclaw", 42, nil)
	if err != nil {
		t.Fatal(err)
	}
	if row["title"] != "Local model" {
		t.Fatalf("row = %#v", row)
	}
	if len(got) < 8 || !slices.Equal(got[:3], []string{"operation", "submit", "issue.issues_get"}) {
		t.Fatalf("args = %v", got)
	}
	var target map[string]any
	if err := json.Unmarshal([]byte(got[4]), &target); err != nil || target["owner"] != "openclaw" || target["repo"] != "openclaw" || target["number"] != float64(42) {
		t.Fatalf("target = %v, err = %v", target, err)
	}
}

func TestBrokerKitListRepositoryIssuesUsesBoundedPages(t *testing.T) {
	t.Parallel()
	pages := []int{}
	client := NewBrokerKit(BrokerKitOptions{Runner: func(_ context.Context, args ...string) ([]byte, error) {
		var arguments map[string]any
		if err := json.Unmarshal([]byte(args[6]), &arguments); err != nil {
			t.Fatal(err)
		}
		pages = append(pages, int(arguments["page"].(float64)))
		return []byte(`{"state":"succeeded","result":[{"id":1,"number":1,"state":"open"},{"id":2,"number":2,"state":"open"}]}`), nil
	}})
	rows, err := client.ListRepositoryIssues(context.Background(), "openclaw", "openclaw", ListIssuesOptions{State: "open"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !slices.Equal(pages, []int{1}) {
		t.Fatalf("rows = %v, pages = %v", rows, pages)
	}
}

func TestBrokerKitFailsClosed(t *testing.T) {
	t.Parallel()
	client := NewBrokerKit(BrokerKitOptions{Runner: func(context.Context, ...string) ([]byte, error) {
		return []byte(`{"state":"denied","error":{"message":"policy denied"}}`), nil
	}})
	if _, err := client.GetRepo(context.Background(), "openclaw", "openclaw", nil); err == nil {
		t.Fatal("denied operation accepted")
	}
	if _, err := client.ListIssueComments(context.Background(), "openclaw", "openclaw", 1, nil); !errors.Is(err, ErrBrokerKitUnsupported) {
		t.Fatalf("unsupported error = %v", err)
	}
}
