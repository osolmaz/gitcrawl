package cli

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"
)

func cacheableGHRead(args []string) bool {
	if len(args) == 0 || hasAnyGHFlag(args, "--web", "--browser", "--interactive") {
		return false
	}
	switch args[0] {
	case "api":
		return ghAPIReadOnly(args[1:])
	case "cache":
		return len(args) >= 2 && args[1] == "list"
	case "gist":
		return len(args) >= 2 && (args[1] == "list" || args[1] == "view")
	case "label":
		return len(args) >= 2 && args[1] == "list"
	case "org":
		return len(args) >= 2 && args[1] == "list"
	case "project":
		return len(args) >= 2 && (args[1] == "list" || args[1] == "view" || args[1] == "field-list" || args[1] == "item-list")
	case "run":
		return len(args) >= 2 && (args[1] == "list" || args[1] == "view")
	case "pr":
		return len(args) >= 2 && (args[1] == "diff" || args[1] == "checks" || args[1] == "list" || args[1] == "status" || args[1] == "view")
	case "issue":
		return len(args) >= 2 && (args[1] == "list" || args[1] == "status" || args[1] == "view")
	case "release":
		return len(args) >= 2 && (args[1] == "list" || args[1] == "view")
	case "repo":
		return len(args) >= 2 && (args[1] == "view" || args[1] == "list")
	case "ruleset":
		return len(args) >= 2 && (args[1] == "check" || args[1] == "list" || args[1] == "view")
	case "search":
		return len(args) >= 2 && (args[1] == "code" || args[1] == "commits" || args[1] == "issues" || args[1] == "prs" || args[1] == "repos")
	case "secret":
		return len(args) >= 2 && args[1] == "list"
	case "variable":
		return len(args) >= 2 && (args[1] == "get" || args[1] == "list")
	case "workflow":
		return len(args) >= 2 && (args[1] == "list" || args[1] == "view")
	default:
		return false
	}
}

func ghCommandName(args []string) string {
	if len(args) == 0 {
		return ""
	}
	if len(args) == 1 {
		return args[0]
	}
	return args[0] + " " + args[1]
}

func ghAPIReadOnly(args []string) bool {
	method := "GET"
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--input", "-F", "-f", "--field", "--raw-field":
			return false
		case "--method", "-X":
			if index+1 >= len(args) {
				return false
			}
			method = strings.ToUpper(args[index+1])
			index++
		default:
			if strings.HasPrefix(arg, "--method=") {
				method = strings.ToUpper(strings.TrimPrefix(arg, "--method="))
			}
		}
	}
	return method == "GET"
}

func (a *App) ghCommandCacheTTL(ctx context.Context, args []string) time.Duration {
	return ghCommandCacheTTLBase(args, a.ghCommandStableIdentity(ctx, args) != "")
}

func ghCommandCacheTTL(args []string) time.Duration {
	return ghCommandCacheTTLBase(args, false)
}

func ghCommandCacheTTLBase(args []string, stablePRDiff bool) time.Duration {
	if raw := strings.TrimSpace(os.Getenv("GITCRAWL_GH_CACHE_TTL")); raw != "" {
		if duration, err := time.ParseDuration(raw); err == nil && duration > 0 {
			return duration
		}
	}
	if len(args) >= 2 {
		if args[0] == "pr" && args[1] == "diff" {
			if stablePRDiff {
				return 7 * 24 * time.Hour
			}
			return 5 * time.Minute
		}
		if args[0] == "api" {
			return time.Minute
		}
	}
	return 30 * time.Second
}

func isGHPRDiff(args []string) bool {
	return len(args) >= 2 && args[0] == "pr" && args[1] == "diff"
}

func parseGHPRDiffIdentityArgs(args []string) (string, int, bool) {
	if !isGHPRDiff(args) {
		return "", 0, false
	}
	var repo string
	var number int
	for index := 2; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "-R", "--repo":
			if index+1 >= len(args) {
				return "", 0, false
			}
			repo = strings.TrimSpace(args[index+1])
			index++
		default:
			if strings.HasPrefix(arg, "--repo=") {
				repo = strings.TrimSpace(strings.TrimPrefix(arg, "--repo="))
				continue
			}
			if strings.HasPrefix(arg, "-") || number != 0 {
				continue
			}
			parsed, err := parseThreadNumber(arg)
			if err != nil {
				return "", 0, false
			}
			number = parsed
		}
	}
	if repo == "" {
		if envRepo := strings.TrimSpace(os.Getenv("GH_REPO")); envRepo != "" {
			repo = envRepo
		}
	}
	return repo, number, repo != "" && number > 0
}

func ghPRHeadSHAFromRawJSON(raw string) string {
	var payload struct {
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Head.SHA)
}

func mutatingGHCommand(args []string) bool {
	if len(args) < 2 {
		return false
	}
	switch args[0] {
	case "cache":
		return args[1] == "delete"
	case "gist":
		switch args[1] {
		case "create", "delete", "edit":
			return true
		}
	case "issue":
		switch args[1] {
		case "close", "comment", "create", "delete", "edit", "lock", "pin", "reopen", "transfer", "unlock", "unpin":
			return true
		}
	case "label":
		switch args[1] {
		case "clone", "create", "delete", "edit":
			return true
		}
	case "pr":
		switch args[1] {
		case "checkout":
			return false
		case "close", "comment", "create", "edit", "lock", "merge", "ready", "reopen", "review", "unlock":
			return true
		}
	case "project":
		switch args[1] {
		case "close", "copy", "create", "delete", "edit", "field-create", "field-delete", "item-add", "item-archive", "item-create", "item-delete", "item-edit", "link", "mark-template", "unlink":
			return true
		}
	case "release":
		switch args[1] {
		case "create", "delete", "delete-asset", "edit", "upload":
			return true
		}
	case "repo":
		switch args[1] {
		case "archive", "create", "delete", "edit", "fork", "rename", "sync":
			return true
		}
	case "ruleset":
		return args[1] == "delete"
	case "run":
		switch args[1] {
		case "cancel", "delete", "rerun":
			return true
		}
	case "secret":
		switch args[1] {
		case "delete", "remove", "set":
			return true
		}
	case "variable":
		switch args[1] {
		case "delete", "remove", "set":
			return true
		}
	case "workflow":
		switch args[1] {
		case "disable", "enable", "run":
			return true
		}
	case "api":
		return !ghAPIReadOnly(args[1:])
	}
	return false
}

func hasAnyGHFlag(args []string, flags ...string) bool {
	for _, arg := range args {
		for _, flag := range flags {
			if arg == flag || strings.HasPrefix(arg, flag+"=") {
				return true
			}
		}
	}
	return false
}
