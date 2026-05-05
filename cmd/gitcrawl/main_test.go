package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainPrintsVersion(t *testing.T) {
	oldArgs := os.Args
	oldStdout := os.Stdout
	t.Cleanup(func() {
		os.Args = oldArgs
		os.Stdout = oldStdout
	})
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = write
	os.Args = []string{"gitcrawl", "--version"}
	main()
	if err := write.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	var out bytes.Buffer
	if _, err := out.ReadFrom(read); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if out.String() == "" {
		t.Fatal("version output was empty")
	}
}

func TestMainUsesGHShimWhenBinaryNameIsGH(t *testing.T) {
	oldArgs := os.Args
	oldStdout := os.Stdout
	t.Cleanup(func() {
		os.Args = oldArgs
		os.Stdout = oldStdout
	})
	dir := t.TempDir()
	ghPath := filepath.Join(dir, "real-gh")
	if err := os.WriteFile(ghPath, []byte("#!/bin/sh\necho shim-fallback:$*\n"), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("GITCRAWL_GH_PATH", ghPath)
	t.Setenv("GITCRAWL_CONFIG", filepath.Join(dir, "config.toml"))
	t.Setenv("GH_REPO", "openclaw/openclaw")

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = write
	os.Args = []string{filepath.Join(dir, "gh"), "run", "view", "123"}
	main()
	if err := write.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	var out bytes.Buffer
	if _, err := out.ReadFrom(read); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "shim-fallback:run view 123" {
		t.Fatalf("output = %q", got)
	}
}
