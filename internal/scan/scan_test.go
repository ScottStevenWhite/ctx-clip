package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openai/ctx-clip/internal/matcher"
)

func TestCollectRespectsFiltersDepthAndHiddenFiles(t *testing.T) {
	dir := t.TempDir()
	mustWriteScanFile(t, filepath.Join(dir, "README.md"), "root\n")
	mustWriteScanFile(t, filepath.Join(dir, ".hidden.md"), "secret\n")
	mustWriteScanFile(t, filepath.Join(dir, "docs", "guide.md"), "guide\n")
	mustWriteScanFile(t, filepath.Join(dir, "docs", ".draft.md"), "draft\n")
	mustWriteScanFile(t, filepath.Join(dir, "docs", "nested", "deep.md"), "deep\n")
	mustWriteScanFile(t, filepath.Join(dir, "node_modules", "pkg", "index.js"), "bundle\n")

	include, err := matcher.Compile([]string{"**/*.md"})
	if err != nil {
		t.Fatalf("Compile(include) error = %v", err)
	}
	exclude, err := matcher.Compile([]string{"node_modules"})
	if err != nil {
		t.Fatalf("Compile(exclude) error = %v", err)
	}

	withWorkingDir(t, dir, func() {
		files, stats, err := Collect(Options{
			Roots:         []string{"."},
			MaxDepth:      1,
			Include:       include,
			Exclude:       exclude,
			IncludeHidden: false,
		})
		if err != nil {
			t.Fatalf("Collect() error = %v", err)
		}

		if len(files) != 2 {
			t.Fatalf("len(files) = %d, want 2", len(files))
		}
		if files[0].DisplayPath != "README.md" || files[1].DisplayPath != "docs/guide.md" {
			t.Fatalf("unexpected collected files: %#v", files)
		}
		if stats.HiddenSkipped != 2 {
			t.Fatalf("stats.HiddenSkipped = %d, want 2", stats.HiddenSkipped)
		}
		if stats.IgnoredSkipped != 1 {
			t.Fatalf("stats.IgnoredSkipped = %d, want 1", stats.IgnoredSkipped)
		}
	})
}

func TestCollectAllowsExplicitHiddenRoot(t *testing.T) {
	dir := t.TempDir()
	mustWriteScanFile(t, filepath.Join(dir, ".secret.md"), "top secret\n")

	withWorkingDir(t, dir, func() {
		files, stats, err := Collect(Options{
			Roots:         []string{".secret.md"},
			MaxDepth:      -1,
			IncludeHidden: false,
		})
		if err != nil {
			t.Fatalf("Collect() error = %v", err)
		}

		if len(files) != 1 {
			t.Fatalf("len(files) = %d, want 1", len(files))
		}
		if files[0].DisplayPath != ".secret.md" {
			t.Fatalf("files[0].DisplayPath = %q, want .secret.md", files[0].DisplayPath)
		}
		if stats.HiddenSkipped != 0 {
			t.Fatalf("stats.HiddenSkipped = %d, want 0", stats.HiddenSkipped)
		}
	})
}

func TestCollectSymlinkRootFollowOption(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "real")
	mustWriteScanFile(t, filepath.Join(targetDir, "linked.md"), "linked\n")

	linkPath := filepath.Join(dir, "alias")
	if err := os.Symlink(targetDir, linkPath); err != nil {
		t.Skipf("os.Symlink() unsupported: %v", err)
	}

	withWorkingDir(t, dir, func() {
		files, stats, err := Collect(Options{
			Roots:          []string{"alias"},
			MaxDepth:       -1,
			FollowSymlinks: false,
		})
		if err != nil {
			t.Fatalf("Collect() error = %v", err)
		}
		if len(files) != 0 {
			t.Fatalf("len(files) = %d, want 0", len(files))
		}
		if stats.SymlinkDirSkipped != 1 {
			t.Fatalf("stats.SymlinkDirSkipped = %d, want 1", stats.SymlinkDirSkipped)
		}
	})

	withWorkingDir(t, dir, func() {
		files, stats, err := Collect(Options{
			Roots:          []string{"alias"},
			MaxDepth:       -1,
			FollowSymlinks: true,
		})
		if err != nil {
			t.Fatalf("Collect() error = %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("len(files) = %d, want 1", len(files))
		}
		if files[0].DisplayPath != "alias/linked.md" {
			t.Fatalf("files[0].DisplayPath = %q, want alias/linked.md", files[0].DisplayPath)
		}
		if stats.SymlinkDirSkipped != 0 {
			t.Fatalf("stats.SymlinkDirSkipped = %d, want 0", stats.SymlinkDirSkipped)
		}
	})
}

func TestCollectUsesFullPathsWhenRequested(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "docs", "file.md")
	mustWriteScanFile(t, filePath, "content\n")

	withWorkingDir(t, dir, func() {
		files, _, err := Collect(Options{
			Roots:     []string{"docs/file.md"},
			MaxDepth:  -1,
			FullPaths: true,
		})
		if err != nil {
			t.Fatalf("Collect() error = %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("len(files) = %d, want 1", len(files))
		}

		want := strings.ReplaceAll(filepath.Clean(filePath), "\\", "/")
		if files[0].DisplayPath != want {
			t.Fatalf("files[0].DisplayPath = %q, want %q", files[0].DisplayPath, want)
		}
		if files[0].SourcePath != filepath.Clean(filePath) {
			t.Fatalf("files[0].SourcePath = %q, want %q", files[0].SourcePath, filepath.Clean(filePath))
		}
	})
}

func mustWriteScanFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%s) error = %v", dir, err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore Chdir(%s) error = %v", wd, err)
		}
	}()
	fn()
}
