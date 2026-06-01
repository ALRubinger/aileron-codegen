package codegen_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ALRubinger/aileron-codegen/pkg/codegen"
)

// TestGolden runs every case under ../../testdata/<case>/ through Generate
// and verifies the emitted tree matches <case>/expected/ byte-for-byte.
//
// Per the project testing philosophy, this asserts on the codegen contract
// — for a given (spec, gen.yaml) pair, the emitter produces a known output
// tree. It does not inspect internal representation or implementation path.
func TestGolden(t *testing.T) {
	const root = "../../testdata"
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			runCase(t, filepath.Join(root, name))
		})
	}
}

func runCase(t *testing.T, caseDir string) {
	t.Helper()
	outDir := t.TempDir()
	if err := codegen.Generate(codegen.Options{
		SpecPath:    filepath.Join(caseDir, "spec.yaml"),
		OverlayPath: filepath.Join(caseDir, "gen.yaml"),
		OutDir:      outDir,
	}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	expected := snapshot(t, filepath.Join(caseDir, "expected"))
	actual := snapshot(t, outDir)
	diff(t, expected, actual)
}

// snapshot returns a map of slash-separated relative path -> sha256(content)
// for every regular file under dir. Dot-files are skipped so .gitkeep
// markers used to preserve empty directories under version control do not
// register as false-positive diffs.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return out
	} else if err != nil {
		t.Fatalf("stat %s: %v", dir, err)
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return out
}

func diff(t *testing.T, expected, actual map[string]string) {
	t.Helper()
	for path, sum := range expected {
		got, ok := actual[path]
		if !ok {
			t.Errorf("missing expected file: %s", path)
			continue
		}
		if got != sum {
			t.Errorf("content mismatch: %s (expected sha256=%s, got %s)", path, sum, got)
		}
	}
	for path := range actual {
		if _, ok := expected[path]; !ok {
			t.Errorf("unexpected emitted file: %s", path)
		}
	}
}
