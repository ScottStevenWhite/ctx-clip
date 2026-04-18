package ctxconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ctx")
	err := os.WriteFile(path, []byte(`
# comment
include server/src/**
include web/src/**
exclude node_modules|package-lock.json
context adr_example.md supporting.md ../shared/glossary.md
map "space file.md" => "../shared/with spaces.md"
max-depth 2
hidden false
follow-symlinks true
same-filesystem true
full-paths false
clipboard true
print false
`), 0o644)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Include) != 2 {
		t.Fatalf("len(cfg.Include) = %d, want 2", len(cfg.Include))
	}
	if len(cfg.Exclude) != 1 {
		t.Fatalf("len(cfg.Exclude) = %d, want 1", len(cfg.Exclude))
	}
	if len(cfg.Mappings) != 2 {
		t.Fatalf("len(cfg.Mappings) = %d, want 2", len(cfg.Mappings))
	}
	if cfg.Mappings[0].Target != "adr_example.md" {
		t.Fatalf("cfg.Mappings[0].Target = %q, want adr_example.md", cfg.Mappings[0].Target)
	}
	if len(cfg.Mappings[0].Related) != 2 {
		t.Fatalf("len(cfg.Mappings[0].Related) = %d, want 2", len(cfg.Mappings[0].Related))
	}
	if cfg.Mappings[1].Target != "space file.md" {
		t.Fatalf("cfg.Mappings[1].Target = %q, want space file.md", cfg.Mappings[1].Target)
	}
	if got := cfg.Mappings[1].Related[0]; got != "../shared/with spaces.md" {
		t.Fatalf("cfg.Mappings[1].Related[0] = %q, want ../shared/with spaces.md", got)
	}
	if cfg.MaxDepth == nil || *cfg.MaxDepth != 2 {
		t.Fatalf("cfg.MaxDepth = %v, want 2", cfg.MaxDepth)
	}
	if cfg.Hidden == nil || *cfg.Hidden {
		t.Fatalf("cfg.Hidden = %v, want false", cfg.Hidden)
	}
	if cfg.FollowSymlinks == nil || !*cfg.FollowSymlinks {
		t.Fatalf("cfg.FollowSymlinks = %v, want true", cfg.FollowSymlinks)
	}
	if cfg.PrintPayload == nil || *cfg.PrintPayload {
		t.Fatalf("cfg.PrintPayload = %v, want false", cfg.PrintPayload)
	}
}

func TestLoadRejectsBrokenMapping(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ctx")
	err := os.WriteFile(path, []byte(`
context "broken.md
`), 0o644)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err = Load(path)
	if err == nil {
		t.Fatal("expected Load() to fail for broken mapping")
	}
}

func TestLoadPreservesBackslashesInMappings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ctx")
	err := os.WriteFile(path, []byte(`
context adr_example.md docs\supporting.md
`), 0o644)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.Mappings[0].Related[0]; got != `docs\supporting.md` {
		t.Fatalf("cfg.Mappings[0].Related[0] = %q, want %q", got, `docs\supporting.md`)
	}
}
