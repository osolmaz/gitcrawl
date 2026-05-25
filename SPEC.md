# gitcrawl Spec

## Product Contract

`gitcrawl` is a local-first GitHub maintainer triage tool written in Go.

The target is a compact, local SQLite workflow for syncing, searching, clustering, and reviewing related GitHub issues and pull requests.

## In Scope

- local SQLite storage
- metadata-first GitHub sync for open issues and pull requests
- optional comment, review, review-comment, PR code, and PR review-thread hydration
- canonical thread document building
- FTS search
- OpenAI summaries and embeddings
- deterministic fingerprints
- vector search
- clustering and durable cluster governance
- portable sync export/import
- CLI JSON surfaces for automation and agents
- TUI browsing after core JSON contracts settle

## Out Of Scope

- local HTTP API
- hosted service runtime
- browser web UI
- GitHub write-back actions

## Architecture

- `cmd/gitcrawl`: executable entrypoint
- `internal/cli`: command parsing and output
- `internal/config`: config and env resolution
- `internal/store`: SQLite schema and persistence
- `internal/github`: GitHub API client
- `internal/syncer`: repository sync workflows
- `internal/documents`: canonical document generation
- `internal/openai`: OpenAI summaries and embeddings
- `internal/vector`: vector search abstraction
- `internal/cluster`: similarity and durable cluster governance
- `internal/search`: keyword, semantic, and hybrid search
- `internal/portable`: compact sync export/import
- `internal/tui`: terminal UI

TUI guidance:

- `gitcrawl tui [owner/repo]` is a supported command; omit `owner/repo` to use the most recently updated local repository
- keyboard-first navigation is required
- mouse support is optional polish
- right-click must not be required for primary actions because terminal mouse support is inconsistent
- avoid decorative glyph noise or transient rendering debris in dense panes

## Command Surface

No `serve` command.

Public commands:

- `init`
- `doctor`
- `configure`
- `version`
- `sync`
- `refresh`
- `summarize`
- `key-summaries`
- `embed`
- `cluster`
- `threads`
- `runs`
- `clusters`
- `clusters-report`
- `durable-clusters`
- `cluster-detail`
- `cluster-explain`
- `neighbors`
- `search`
- `gh`
- `close-thread`
- `close-cluster`
- `exclude-cluster-member`
- `include-cluster-member`
- `set-cluster-canonical`
- `merge-clusters`
- `split-cluster`
- `export-sync`
- `import-sync`
- `validate-sync`
- `portable-size`
- `sync-status`
- `optimize`
- `tui`
- `completion`

`search` also supports the common `gh search` read-only shape for cached discovery:

```text
gitcrawl search issues <query> -R owner/repo --state open --json number,title,state,url,updatedAt,labels --limit 30
gitcrawl search prs <query> -R owner/repo --state open --json number,title,state,url,updatedAt,isDraft,author --limit 20
gitcrawl search issues <query> -R owner/repo --state open --sync-if-stale 5m --json number,title,url
```

This compatibility path reads from local SQLite by default. It avoids GitHub REST search quota and is not a replacement for final live `gh` verification before comments, closes, labels, or merges. `--sync-if-stale <duration>` may run one metadata sync first when the repository mirror is older than the requested max age; the search result itself still comes from SQLite.

`gh` is the agent-facing compatibility shim. It may be invoked as `gitcrawl gh ...` or by installing the binary as `gh`/`gitcrawl-gh`. Supported local reads:

```text
gitcrawl gh search issues|prs <query> -R owner/repo --state open --match comments --json number,title,url
gitcrawl gh issue view 123 -R owner/repo --json number,title,state,url,body
gitcrawl gh pr view 123 -R owner/repo --json number,title,state,url,isDraft,author
gitcrawl gh pr status 123 -R owner/repo --compact
gitcrawl gh issue list -R owner/repo --state open --search "hot loop" --json number,title,url
gitcrawl gh pr list -R owner/repo --state open --search "manifest cache" --json number,title,url
```

`gitcrawl gh pr status` is the first read for PR triage. It returns a compact readiness summary from local SQLite when possible, including cached check state, approval/changes-requested review state, unresolved review-thread counts, cache age, and blocking reasons. Exit code `0` means clean, `1` means action needed, `2` means error, and `3` means pending.

Unsupported commands fall through to the real GitHub CLI. Read-only fallthroughs use a command-aware persistent cache in `cache/gh-shim` for repeated agent calls (`run list/view`, `pr diff/list/view/checks/status`, `issue list/status/view`, `repo view/list`, `release list/view`, `workflow list/view`, `secret list`, `variable get/list`, `project` list/view reads, `ruleset` reads, `gist` reads, `org list`, `label list`, read-only `search` kinds, and GET-only `api`). Actions run/job logs are cached much longer than CI status reads, completed run reads receive longer TTLs, and `xcache stats` records hit rate plus backend misses by command and normalized route so remaining GitHub-heavy patterns are visible. Repeat read failures are cached by default so many agents do not rediscover the same missing release, workflow, or field, with short caps for error entries and rate-limit responses; if GitHub rate-limits a refresh and a stale successful entry exists, the stale entry is served with a warning. Set `GITCRAWL_GH_CACHE_ERRORS=0` to disable error caching. Mutating commands are never cached and invalidate matching cache-tag entries on success. Unknown mutation scope falls back to clearing the fallthrough cache. The shim does not add GitHub write-back behavior of its own; writes remain delegated to `gh`.

Cache inspection commands:

```text
gitcrawl gh xcache stats
gitcrawl gh xcache keys
gitcrawl gh xcache reset
gitcrawl gh xcache flush
gitcrawl gh xcache snapshot [--reset]
```

The cache key includes the resolved gitcrawl config path, current working directory, `GH_HOST`, `GH_REPO`, stable PR-diff identity when available, and canonicalized `gh` arguments. This keeps sibling checkouts and portable stores isolated while still coalescing equivalent agent calls such as reordered flags or sorted `--json` fields. Concurrent cache misses use a lock file so one process populates the entry while peers wait for the result; if an expired successful entry is still inside its stale grace window, peers may serve stale while the lock holder refreshes it. `xcache stats --since <duration>` reports recent-window counters from hourly buckets, and miss maps include command, normalized route, and canonical key views.

## Config

Default config path:

```text
~/.config/gitcrawl/config.toml
```

Default database path:

```text
~/.config/gitcrawl/gitcrawl.db
```

Primary environment variables:

- `GITCRAWL_CONFIG`
- `GITHUB_TOKEN`
- `OPENAI_API_KEY`
- `GITCRAWL_DB_PATH`
- `GITCRAWL_SUMMARY_MODEL`
- `GITCRAWL_EMBED_MODEL`

Legacy environment aliases may be supported only when they do not leak old naming into user-facing output.
