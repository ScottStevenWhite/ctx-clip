package ctxconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openai/ctx-clip/internal/scan"
)

func TestExpandMappedFilesRecursiveAndMissing(t *testing.T) {
	dir := t.TempDir()

	mustWriteFile(t, filepath.Join(dir, ".ctx"), `
context adr_example.md supporting.md missing.txt
context supporting.md glossary.md
`)
	mustWriteFile(t, filepath.Join(dir, "adr_example.md"), "# ADR\n")
	mustWriteFile(t, filepath.Join(dir, "supporting.md"), "support\n")
	mustWriteFile(t, filepath.Join(dir, "glossary.md"), "terms\n")

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		if chdirErr := os.Chdir(wd); chdirErr != nil {
			t.Fatalf("restore Chdir() error = %v", chdirErr)
		}
	}()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	files, _, err := scan.Collect(scan.Options{
		Roots:    []string{"adr_example.md"},
		MaxDepth: -1,
	})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	expanded, stats, warnings, err := ExpandMappedFiles(files, false)
	if err != nil {
		t.Fatalf("ExpandMappedFiles() error = %v", err)
	}

	if len(expanded) != 3 {
		t.Fatalf("len(expanded) = %d, want 3", len(expanded))
	}
	if stats.FilesCopied != 2 {
		t.Fatalf("stats.FilesCopied = %d, want 2", stats.FilesCopied)
	}
	if len(warnings) != 1 {
		t.Fatalf("len(warnings) = %d, want 1", len(warnings))
	}
	if !strings.Contains(warnings[0], "missing.txt was not found") {
		t.Fatalf("warning = %q, want missing file text", warnings[0])
	}

	gotPaths := []string{
		expanded[0].DisplayPath,
		expanded[1].DisplayPath,
		expanded[2].DisplayPath,
	}
	want := []string{"adr_example.md", "glossary.md", "supporting.md"}
	for i := range want {
		if gotPaths[i] != want[i] {
			t.Fatalf("expanded[%d].DisplayPath = %q, want %q", i, gotPaths[i], want[i])
		}
	}
}

func TestExpandMappedFilesRegexCrossDirectoryAndCycle(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	sharedDir := filepath.Join(root, "shared")

	mustWriteFile(t, filepath.Join(docsDir, ".ctx"), `
context re:^adr_.*\.md$ ../shared/glossary.md
`)
	mustWriteFile(t, filepath.Join(sharedDir, ".ctx"), `
context glossary.md schema.json ../docs/adr_example.md
`)
	mustWriteFile(t, filepath.Join(docsDir, "adr_example.md"), "# ADR\n")
	mustWriteFile(t, filepath.Join(sharedDir, "glossary.md"), "terms\n")
	mustWriteFile(t, filepath.Join(sharedDir, "schema.json"), "{\n  \"type\": \"object\"\n}\n")

	files := collectFromDir(t, root, []string{"docs/adr_example.md"})

	expanded, stats, warnings, err := ExpandMappedFiles(files, false)
	if err != nil {
		t.Fatalf("ExpandMappedFiles() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("len(warnings) = %d, want 0; warnings=%v", len(warnings), warnings)
	}
	if len(expanded) != 3 {
		t.Fatalf("len(expanded) = %d, want 3", len(expanded))
	}
	if stats.FilesCopied != 2 {
		t.Fatalf("stats.FilesCopied = %d, want 2", stats.FilesCopied)
	}

	gotPaths := []string{
		expanded[0].DisplayPath,
		expanded[1].DisplayPath,
		expanded[2].DisplayPath,
	}
	want := []string{"docs/adr_example.md", "shared/glossary.md", "shared/schema.json"}
	for i := range want {
		if gotPaths[i] != want[i] {
			t.Fatalf("expanded[%d].DisplayPath = %q, want %q", i, gotPaths[i], want[i])
		}
	}
}

func TestExpandMappedFilesWarnsOnBrokenCtxAndKeepsOriginalFiles(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, ".ctx"), `
context "broken.md
`)
	mustWriteFile(t, filepath.Join(dir, "adr_example.md"), "# ADR\n")

	files := collectFromDir(t, dir, []string{"adr_example.md"})

	expanded, stats, warnings, err := ExpandMappedFiles(files, false)
	if err != nil {
		t.Fatalf("ExpandMappedFiles() error = %v", err)
	}
	if len(expanded) != 1 {
		t.Fatalf("len(expanded) = %d, want 1", len(expanded))
	}
	if stats.FilesCopied != 0 {
		t.Fatalf("stats.FilesCopied = %d, want 0", stats.FilesCopied)
	}
	if len(warnings) != 1 {
		t.Fatalf("len(warnings) = %d, want 1", len(warnings))
	}
	if !strings.Contains(warnings[0], "could not load") {
		t.Fatalf("warning = %q, want load warning", warnings[0])
	}
}

func TestExpandMappedFilesWarnsOnDirectoryMapping(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, ".ctx"), `
context adr_example.md supporting
`)
	mustWriteFile(t, filepath.Join(dir, "adr_example.md"), "# ADR\n")
	if err := os.MkdirAll(filepath.Join(dir, "supporting"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	files := collectFromDir(t, dir, []string{"adr_example.md"})

	expanded, stats, warnings, err := ExpandMappedFiles(files, false)
	if err != nil {
		t.Fatalf("ExpandMappedFiles() error = %v", err)
	}
	if len(expanded) != 1 {
		t.Fatalf("len(expanded) = %d, want 1", len(expanded))
	}
	if stats.FilesCopied != 0 {
		t.Fatalf("stats.FilesCopied = %d, want 0", stats.FilesCopied)
	}
	if len(warnings) != 1 {
		t.Fatalf("len(warnings) = %d, want 1", len(warnings))
	}
	if !strings.Contains(warnings[0], "points to a directory") {
		t.Fatalf("warning = %q, want directory warning", warnings[0])
	}
}

func collectFromDir(t *testing.T, dir string, roots []string) []scan.File {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		if chdirErr := os.Chdir(wd); chdirErr != nil {
			t.Fatalf("restore Chdir() error = %v", chdirErr)
		}
	}()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	files, _, err := scan.Collect(scan.Options{
		Roots:    roots,
		MaxDepth: -1,
	})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	return files
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimLeft(content, "\n")), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
