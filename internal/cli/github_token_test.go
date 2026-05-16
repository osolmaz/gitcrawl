package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/openclaw/gitcrawl/internal/config"
)

func TestResolveGitHubTokenFallsBackToGHAuthToken(t *testing.T) {
	dir := t.TempDir()
	ghPath := filepath.Join(dir, "gh")
	if err := os.WriteFile(ghPath, []byte("#!/bin/sh\nif [ \"$1\" = auth ] && [ \"$2\" = token ]; then echo gh-fallback-token; exit 0; fi\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITCRAWL_GH_PATH", ghPath)

	token := New().resolveGitHubToken(context.Background(), config.Default())
	if token.Value != "gh-fallback-token" || token.Source != "gh auth token" {
		t.Fatalf("token = %#v", token)
	}
}

func TestDoctorReportsGHAuthTokenFallback(t *testing.T) {
	dir := t.TempDir()
	ghPath := filepath.Join(dir, "gh")
	if err := os.WriteFile(ghPath, []byte("#!/bin/sh\nif [ \"$1\" = auth ] && [ \"$2\" = token ]; then echo gh-fallback-token; exit 0; fi\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	configPath := filepath.Join(dir, "config.toml")
	dbPath := filepath.Join(dir, "gitcrawl.db")
	if err := os.WriteFile(configPath, []byte("version = 1\ndb_path = "+strconv.Quote(dbPath)+"\n[github]\ntoken_env = 'GITHUB_TOKEN'\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITCRAWL_GH_PATH", ghPath)

	doctor := New()
	var stdout bytes.Buffer
	doctor.Stdout = &stdout
	if err := doctor.Run(context.Background(), []string{"--config", configPath, "doctor", "--json"}); err != nil {
		t.Fatalf("doctor: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("parse doctor json: %v\n%s", err, stdout.String())
	}
	if got := payload["github_token_present"]; got != true {
		t.Fatalf("github_token_present = %#v, payload=%s", got, stdout.String())
	}
	if got := payload["github_token_source"]; got != "gh auth token" {
		t.Fatalf("github_token_source = %#v, payload=%s", got, stdout.String())
	}
}
