package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRun covers the CLI contract: required flags are enforced, valid
// invocations exit 0, and the configured --out directory exists when
// Generate returns.
func TestRun(t *testing.T) {
	spec := writeFile(t, "spec.yaml", "openapi: 3.0.3\ninfo:\n  title: t\n  version: 0.0.0\npaths: {}\n")
	overlay := writeFile(t, "gen.yaml", "operations: {}\n")
	outDir := filepath.Join(t.TempDir(), "out")

	t.Run("happy path", func(t *testing.T) {
		var stderr bytes.Buffer
		err := run([]string{
			"--spec", spec,
			"--overlay", overlay,
			"--out", outDir,
		}, &stderr)
		if err != nil {
			t.Fatalf("run: %v (stderr=%q)", err, stderr.String())
		}
		if _, err := os.Stat(outDir); err != nil {
			t.Fatalf("expected --out to exist: %v", err)
		}
		entries, err := os.ReadDir(outDir)
		if err != nil {
			t.Fatalf("read --out: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("expected no emitted files in scaffolding pass; got %d entries", len(entries))
		}
	})

	t.Run("missing required flag", func(t *testing.T) {
		var stderr bytes.Buffer
		err := run([]string{"--spec", spec, "--overlay", overlay}, &stderr)
		if err == nil {
			t.Fatal("expected error when --out is missing")
		}
		if !strings.Contains(err.Error(), "required") {
			t.Errorf("error %q should mention required flags", err.Error())
		}
	})

	t.Run("unknown flag", func(t *testing.T) {
		var stderr bytes.Buffer
		err := run([]string{"--nope"}, &stderr)
		if err == nil {
			t.Fatal("expected error on unknown flag")
		}
	})
}

func writeFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}
