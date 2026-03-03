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
