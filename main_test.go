package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunUsesAutoCtxConfigInSimulatedMonorepo(t *testing.T) {
	repo := t.TempDir()
	writeRepoFile(t, filepath.Join(repo, ".ctx"), `
include README.md
include docs/**/*.md
include server/src/**
exclude node_modules|dist
hidden false
`)
	writeRepoFile(t, filepath.Join(repo, "README.md"), "# Repo\n")
	writeRepoFile(t, filepath.Join(repo, "docs", "adr_example.md"), "# ADR\n")
	writeRepoFile(t, filepath.Join(repo, "docs", "notes.txt"), "notes\n")
	writeRepoFile(t, filepath.Join(repo, "server", "src", "app.ts"), "console.log('app')\n")
	writeRepoFile(t, filepath.Join(repo, "server", "dist", "bundle.js"), "compiled\n")
	writeRepoFile(t, filepath.Join(repo, "node_modules", "pkg", "index.js"), "pkg\n")
	writeRepoFile(t, filepath.Join(repo, ".hidden.md"), "hidden\n")

	code, stdout, stderr := runInDir(t, repo, "--no-clipboard")
	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0; stderr=%s", code, stderr)
	}

	assertContains(t, stdout, "README.md:\n# Repo\n")
	assertContains(t, stdout, "docs/adr_example.md:\n# ADR\n")
	assertContains(t, stdout, "server/src/app.ts:\nconsole.log('app')\n")
	assertNotContains(t, stdout, "docs/notes.txt:")
	assertNotContains(t, stdout, "server/dist/bundle.js:")
	assertNotContains(t, stdout, "node_modules/pkg/index.js:")
	assertNotContains(t, stdout, ".hidden.md:")
	assertContains(t, stderr, "config: "+filepath.Join(repo, ".ctx"))
	assertContains(t, stderr, "clipboard: disabled")
	assertContains(t, stderr, "ctx-warnings=0")
}

func TestRunExpandCtxSimulatesAdrWorkflow(t *testing.T) {
	repo := t.TempDir()
	writeRepoFile(t, filepath.Join(repo, "docs", ".ctx"), `
context adr_example.md supporting.md ../shared/glossary.md missing.txt
`)
	writeRepoFile(t, filepath.Join(repo, "shared", ".ctx"), `
context glossary.md schema.json
`)
	writeRepoFile(t, filepath.Join(repo, "docs", "adr_example.md"), "# ADR\n")
	writeRepoFile(t, filepath.Join(repo, "docs", "supporting.md"), "support\n")
	writeRepoFile(t, filepath.Join(repo, "shared", "glossary.md"), "terms\n")
	writeRepoFile(t, filepath.Join(repo, "shared", "schema.json"), "{\n  \"type\": \"object\"\n}\n")

	code, stdout, stderr := runInDir(t, repo, "--no-clipboard", "--expand-ctx", "docs/adr_example.md")
	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0; stderr=%s", code, stderr)
	}

	assertContains(t, stdout, "docs/adr_example.md:\n# ADR\n")
	assertContains(t, stdout, "docs/supporting.md:\nsupport\n")
	assertContains(t, stdout, "shared/glossary.md:\nterms\n")
	assertContains(t, stdout, "shared/schema.json:\n{\n  \"type\": \"object\"\n}\n")
	assertContains(t, stderr, "missing.txt was not found")
	assertContains(t, stderr, "ctx-warnings=1")
}

func TestRunNoCtxIgnoresAutoLoadedConfig(t *testing.T) {
	repo := t.TempDir()
	writeRepoFile(t, filepath.Join(repo, ".ctx"), `
include docs/**/*.md
`)
	writeRepoFile(t, filepath.Join(repo, "README.md"), "# Repo\n")
	writeRepoFile(t, filepath.Join(repo, "docs", "adr_example.md"), "# ADR\n")

	code, stdout, stderr := runInDir(t, repo, "--no-clipboard", "--no-ctx")
	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0; stderr=%s", code, stderr)
	}

	assertContains(t, stdout, "README.md:\n# Repo\n")
	assertContains(t, stdout, "docs/adr_example.md:\n# ADR\n")
	assertNotContains(t, stderr, "config:")
}

func TestRunUsesExplicitCtxPath(t *testing.T) {
	repo := t.TempDir()
	configPath := filepath.Join(repo, "configs", "project.ctx")
	writeRepoFile(t, configPath, `
include docs/**/*.md
exclude *.txt
`)
	writeRepoFile(t, filepath.Join(repo, "docs", "adr_example.md"), "# ADR\n")
	writeRepoFile(t, filepath.Join(repo, "docs", "scratch.txt"), "ignore me\n")

	code, stdout, stderr := runInDir(t, repo, "--no-clipboard", "--ctx", configPath)
	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0; stderr=%s", code, stderr)
	}

	assertContains(t, stdout, "docs/adr_example.md:\n# ADR\n")
	assertNotContains(t, stdout, "docs/scratch.txt:")
	assertContains(t, stderr, "config: "+configPath)
}

func TestRunQuietSuppressesPayloadEvenWithNoClipboard(t *testing.T) {
	repo := t.TempDir()
	writeRepoFile(t, filepath.Join(repo, "file.txt"), "content\n")

	code, stdout, stderr := runInDir(t, repo, "--no-clipboard", "--quiet", "file.txt")
	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0; stderr=%s", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertContains(t, stderr, "copied 1 file(s)")
	assertContains(t, stderr, "clipboard: disabled")
}

func TestRunRejectsInvalidIncludePattern(t *testing.T) {
	repo := t.TempDir()
	writeRepoFile(t, filepath.Join(repo, "file.txt"), "content\n")

	code, _, stderr := runInDir(t, repo, "--no-clipboard", "-P", "re:[")
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2; stderr=%s", code, stderr)
	}
	assertContains(t, stderr, "invalid include pattern")
}

func runInDir(t *testing.T, dir string, args ...string) (int, string, string) {
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

	origStdout := os.Stdout
	origStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdout) error = %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stderr) error = %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW
	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})

	go copyPipe(&stdoutBuf, stdoutR, stdoutDone)
	go copyPipe(&stderrBuf, stderrR, stderrDone)

	code := run(args)

	if err := stdoutW.Close(); err != nil {
		t.Fatalf("stdout close error = %v", err)
	}
	if err := stderrW.Close(); err != nil {
		t.Fatalf("stderr close error = %v", err)
	}
	<-stdoutDone
	<-stderrDone

	return code, stdoutBuf.String(), stderrBuf.String()
}

func copyPipe(dst *bytes.Buffer, src *os.File, done chan struct{}) {
	_, _ = io.Copy(dst, src)
	_ = src.Close()
	close(done)
}

func writeRepoFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimLeft(content, "\n")), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func assertContains(t *testing.T, got string, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected output to contain %q\nfull output:\n%s", want, got)
	}
}

func assertNotContains(t *testing.T, got string, unwanted string) {
	t.Helper()
	if strings.Contains(got, unwanted) {
		t.Fatalf("expected output not to contain %q\nfull output:\n%s", unwanted, got)
	}
}
