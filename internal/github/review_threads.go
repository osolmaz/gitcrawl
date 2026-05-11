package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const pullReviewThreadsQuery = `
query($owner: String!, $repo: String!, $pr: Int!, $cursor: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $pr) {
      reviewThreads(first: 100, after: $cursor) {
        nodes {
          id
          isResolved
          isOutdated
          path
          line
          startLine
          viewerCanResolve
          viewerCanUnresolve
          viewerCanReply
          comments(first: 50) {
            nodes {
              id
              databaseId
              body
              author { login __typename }
              path
              diffHunk
              createdAt
              updatedAt
              url
            }
          }
        }
        pageInfo {
          hasNextPage
          endCursor
        }
      }
    }
  }
}
`

type pullReviewThreadsResponse struct {
	Repository struct {
		PullRequest *struct {
			ReviewThreads struct {
				Nodes    []map[string]any `json:"nodes"`
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
			} `json:"reviewThreads"`
		} `json:"pullRequest"`
	} `json:"repository"`
}

type graphqlEnvelope struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type graphqlResponseEnvelope struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// ListPullReviewThreads fetches GitHub's review-thread graph for a pull request.
func (c *Client) ListPullReviewThreads(ctx context.Context, owner, repo string, number int, reporter Reporter) ([]map[string]any, error) {
	var out []map[string]any
	var cursor string
	for {
		vars := map[string]any{
			"owner": owner,
			"repo":  repo,
			"pr":    number,
		}
		if cursor != "" {
			vars["cursor"] = cursor
		}
		var resp pullReviewThreadsResponse
		if err := c.doGraphQL(ctx, pullReviewThreadsQuery, vars, reporter, &resp); err != nil {
			return nil, err
		}
		if resp.Repository.PullRequest == nil {
			return nil, fmt.Errorf("pull request #%d not found in %s/%s", number, owner, repo)
		}
		page := resp.Repository.PullRequest.ReviewThreads
		out = append(out, page.Nodes...)
		if !page.PageInfo.HasNextPage {
			break
		}
		cursor = page.PageInfo.EndCursor
	}
	return out, nil
}

func (c *Client) doGraphQL(ctx context.Context, query string, variables map[string]any, reporter Reporter, out any) error {
	payload, err := json.Marshal(graphqlEnvelope{Query: query, Variables: variables})
	if err != nil {
		return fmt.Errorf("encode graphql request: %w", err)
	}
	var envelope graphqlResponseEnvelope
	if err := c.doJSON(ctx, http.MethodPost, "/graphql", bytes.NewReader(payload), reporter, &envelope); err != nil {
		return err
	}
	if len(envelope.Errors) > 0 {
		messages := make([]string, 0, len(envelope.Errors))
		for _, graphqlErr := range envelope.Errors {
			messages = append(messages, graphqlErr.Message)
		}
		return fmt.Errorf("github graphql: %s", strings.Join(messages, "; "))
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("github graphql response missing data")
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode github graphql data: %w", err)
	}
	return nil
}
