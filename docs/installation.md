---
title: Installation
nav_order: 2
permalink: /installation/
---

# Installation
{: .no_toc }

1. TOC
{:toc}

## Requirements

- **Go 1.26+** if building from source
- **Git** for cloning the repository (and for portable stores)
- **A GitHub token** for any command that talks to GitHub (`sync`, `refresh`, `gh` shim fallthroughs)
- **An OpenAI API key** only for `embed`, `refresh` (embed stage), and any future summary commands
- **`gh` CLI** if you want the shim to fall through to the real GitHub CLI for unsupported commands

gitcrawl runs on macOS and Linux. Windows is not actively tested.

## Install from Homebrew

```bash
brew install openclaw/tap/gitcrawl
```

Homebrew installs the `gitcrawl` binary. If you also want the GitHub CLI shim behavior, add a `gh` or `gitcrawl-gh` symlink as shown below.

## Install from a GitHub release

Each tagged release publishes archives for `darwin_amd64`, `darwin_arm64`, `linux_amd64`, and `linux_arm64` via [GoReleaser](https://github.com/openclaw/gitcrawl/blob/main/.goreleaser.yaml).

```bash
# Replace VERSION and PLATFORM with the values you want.
VERSION=v0.1.2
PLATFORM=darwin_arm64
mkdir -p "$HOME/bin"
curl -L "https://github.com/openclaw/gitcrawl/releases/download/${VERSION}/gitcrawl_${VERSION#v}_${PLATFORM}.tar.gz" \
  | tar -xz -C "$HOME/bin" gitcrawl

gitcrawl --version
```

Browse the [releases page](https://github.com/openclaw/gitcrawl/releases) for the latest tag and the full asset list. Use a directory that is already on your `PATH`; `~/bin` and `~/.local/bin` avoid needing elevated permissions.

## Check for updates

```bash
gitcrawl check-update
gitcrawl check-update --json
```

Interactive terminal runs perform a cached daily release check and print a
stderr notice when a newer OpenClaw release is available. Scripted, JSON, CI,
and non-TTY runs skip the passive notice. Set `GITCRAWL_NO_UPDATE_CHECK=1` or
`CRAWLKIT_NO_UPDATE_CHECK=1` to disable it.

## Install from source

```bash
git clone https://github.com/openclaw/gitcrawl.git
cd gitcrawl
go build \
  -ldflags "-X github.com/openclaw/gitcrawl/internal/cli.version=$(git describe --tags --always --dirty)" \
  -o bin/gitcrawl ./cmd/gitcrawl

./bin/gitcrawl --version
```

Symlink or copy `bin/gitcrawl` somewhere on your `PATH` (`~/bin`, `/usr/local/bin`, `~/.local/bin`).

## Install the `gh` shim

The shim is the same binary. Symlink it as `gh` (replacing the real CLI) or as `gitcrawl-gh` (running side by side):

```bash
# Side-by-side install — agents can opt in by calling `gitcrawl-gh`.
mkdir -p "$HOME/bin"
ln -sf "$(command -v gitcrawl)" "$HOME/bin/gitcrawl-gh"

# Or replace the global `gh` so every agent picks up the cache automatically.
REAL_GH="$(command -v gh)"              # capture this before shadowing gh
ln -sf "$(command -v gitcrawl)" "$HOME/bin/gh"
export GITCRAWL_GH_PATH="$REAL_GH"      # point shim at the real gh
```

When invoked as `gh` or `gitcrawl-gh`, the binary auto-detects shim mode. See [the gh shim guide](/gh-shim/) for details.

## Verify the install

```bash
gitcrawl init           # creates ~/.config/gitcrawl/{config.toml,gitcrawl.db,...}
gitcrawl doctor         # confirms config, database, and credential discovery
gitcrawl doctor --json  # same, machine-readable
```

`doctor` reports whether `GITHUB_TOKEN` and `OPENAI_API_KEY` are present, where they came from, the version, repository count, and the last sync timestamp. If anything is missing, the message tells you which env var or config field to set.

## Updating

- **Release archives:** download the new tarball and replace the binary.
- **Source builds:** `git pull && go build ...` — the version string comes from `git describe`.
- **Configuration is forward-compatible.** Existing `config.toml` and `gitcrawl.db` files are reused across versions; no migration step is needed for normal point releases.
