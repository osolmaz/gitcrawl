package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestGHShimViewAndListUseLocalCache(t *testing.T) {
	ctx := context.Background()
	configPath := seedGHShimRepo(t, ctx)

	run := New()
	var stdout bytes.Buffer
	run.Stdout = &stdout
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "pr", "view", "12", "-R", "openclaw/openclaw", "--json", "number,title,isDraft,author"}); err != nil {
		t.Fatalf("gh pr view: %v", err)
	}
	var view map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &view); err != nil {
		t.Fatalf("decode view: %v\n%s", err, stdout.String())
	}
	if int(view["number"].(float64)) != 12 || view["isDraft"] != true {
		t.Fatalf("view = %#v", view)
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
	if err := run.Run(ctx, []string{"--config", configPath, "gh", "issue", "list", "-R", "openclaw/openclaw", "--author", "alice", "--assignee", "peter", "--label", "bug", "--json", "number,title"}); err != nil {
		t.Fatalf("gh issue list filtered: %v", err)
	}
	if err := json.Unmarshal(stdout.Bytes(), &list); err != nil {
		t.Fatalf("decode filtered list: %v\n%s", err, stdout.String())
	}
	if len(list) != 1 || int(list[0]["number"].(float64)) != 10 {
		t.Fatalf("filtered list = %#v", list)
	}
}
