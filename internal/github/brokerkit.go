package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var ErrBrokerKitUnsupported = errors.New("Gitcrawl BrokerKit transport does not support this hydration mode")

type BrokerKitRunner func(context.Context, ...string) ([]byte, error)

type BrokerKitOptions struct {
	Command string
	Runner  BrokerKitRunner
}

type BrokerKitClient struct {
	run BrokerKitRunner
}

type brokerKitOperation struct {
	State  string          `json:"state"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

func NewBrokerKit(options BrokerKitOptions) *BrokerKitClient {
	runner := options.Runner
	if runner == nil {
		command := strings.TrimSpace(options.Command)
		if command == "" {
			command = "gh-broker"
		}
		runner = func(ctx context.Context, args ...string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, command, args...)
			return cmd.CombinedOutput()
		}
	}
	return &BrokerKitClient{run: runner}
}

func (c *BrokerKitClient) operation(ctx context.Context, name string, target, arguments any, out any) error {
	targetJSON, err := json.Marshal(target)
	if err != nil {
		return fmt.Errorf("encode BrokerKit target: %w", err)
	}
	argumentsJSON, err := json.Marshal(arguments)
	if err != nil {
		return fmt.Errorf("encode BrokerKit arguments: %w", err)
	}
	data, err := c.run(ctx, "operation", "submit", name,
		"--target-json", string(targetJSON), "--arguments-json", string(argumentsJSON), "--wait")
	if err != nil {
		message := strings.TrimSpace(string(data))
		if message == "" {
			return fmt.Errorf("BrokerKit operation %s failed: %w", name, err)
		}
		return fmt.Errorf("BrokerKit operation %s failed: %s", name, message)
	}
	var operation brokerKitOperation
	if err := json.Unmarshal(data, &operation); err != nil {
		return fmt.Errorf("decode BrokerKit operation %s: %w", name, err)
	}
	if operation.State != "succeeded" {
		return fmt.Errorf("BrokerKit operation %s ended in state %q", name, operation.State)
	}
	if len(operation.Result) == 0 || string(operation.Result) == "null" {
		return fmt.Errorf("BrokerKit operation %s returned no result", name)
	}
	if err := json.Unmarshal(operation.Result, out); err != nil {
		return fmt.Errorf("decode BrokerKit result %s: %w", name, err)
	}
	return nil
}

func (c *BrokerKitClient) GetRepo(ctx context.Context, owner, repo string, reporter Reporter) (map[string]any, error) {
	reporter.Printf("[brokerkit] repo.metadata.read %s/%s", owner, repo)
	var out map[string]any
	err := c.operation(ctx, "repo.metadata.read", map[string]any{"kind": "repo", "owner": owner, "name": repo}, map[string]any{}, &out)
	return out, err
}

func (c *BrokerKitClient) GetIssue(ctx context.Context, owner, repo string, number int, reporter Reporter) (map[string]any, error) {
	reporter.Printf("[brokerkit] issue.issues_get %s/%s#%d", owner, repo, number)
	var out map[string]any
	err := c.operation(ctx, "issue.issues_get", map[string]any{"kind": "issue", "owner": owner, "repo": repo, "number": number}, map[string]any{}, &out)
	return out, err
}

func (c *BrokerKitClient) GetPull(ctx context.Context, owner, repo string, number int, reporter Reporter) (map[string]any, error) {
	reporter.Printf("[brokerkit] pull_request.get %s/%s#%d", owner, repo, number)
	var out map[string]any
	err := c.operation(ctx, "pull_request.get", map[string]any{"kind": "pull_request", "owner": owner, "repo": repo, "number": number}, map[string]any{}, &out)
	return out, err
}

func (c *BrokerKitClient) ListRepositoryIssues(ctx context.Context, owner, repo string, options ListIssuesOptions, reporter Reporter) ([]map[string]any, error) {
	state := strings.TrimSpace(options.State)
	if state == "" {
		state = "open"
	}
	var out []map[string]any
	for page := 1; ; page++ {
		arguments := map[string]any{"state": state, "sort": "updated", "direction": "desc", "per_page": 100, "page": page}
		if options.Since != "" {
			arguments["since"] = options.Since
		}
		var rows []map[string]any
		if err := c.operation(ctx, "issue.issues_list_for_repo", map[string]any{"kind": "repo", "owner": owner, "name": repo}, arguments, &rows); err != nil {
			return nil, err
		}
		if options.Limit > 0 && len(out)+len(rows) > options.Limit {
			rows = rows[:options.Limit-len(out)]
		}
		out = append(out, rows...)
		reporter.Printf("[brokerkit] page %d fetched count=%d accumulated=%d", page, len(rows), len(out))
		if len(rows) < 100 || options.Limit > 0 && len(out) >= options.Limit {
			break
		}
	}
	return out, nil
}

func brokerKitHydrationUnsupported(name string) error {
	return fmt.Errorf("%w: %s", ErrBrokerKitUnsupported, name)
}

func (c *BrokerKitClient) ListIssueComments(context.Context, string, string, int, Reporter) ([]map[string]any, error) {
	return nil, brokerKitHydrationUnsupported("issue comments")
}

func (c *BrokerKitClient) ListPullReviews(context.Context, string, string, int, Reporter) ([]map[string]any, error) {
	return nil, brokerKitHydrationUnsupported("pull reviews")
}

func (c *BrokerKitClient) ListPullReviewComments(context.Context, string, string, int, Reporter) ([]map[string]any, error) {
	return nil, brokerKitHydrationUnsupported("pull review comments")
}

func (c *BrokerKitClient) ListPullReviewThreads(context.Context, string, string, int, Reporter) ([]map[string]any, error) {
	return nil, brokerKitHydrationUnsupported("pull review threads")
}

func (c *BrokerKitClient) ListPullFiles(context.Context, string, string, int, Reporter) ([]map[string]any, error) {
	return nil, brokerKitHydrationUnsupported("pull files")
}

func (c *BrokerKitClient) ListPullCommits(context.Context, string, string, int, Reporter) ([]map[string]any, error) {
	return nil, brokerKitHydrationUnsupported("pull commits")
}

func (c *BrokerKitClient) ListCommitCheckRuns(context.Context, string, string, string, Reporter) ([]map[string]any, error) {
	return nil, brokerKitHydrationUnsupported("commit check runs")
}

func (c *BrokerKitClient) ListWorkflowRuns(context.Context, string, string, ListWorkflowRunsOptions, Reporter) ([]map[string]any, error) {
	return nil, brokerKitHydrationUnsupported("workflow runs")
}
