package textfile

import (
    "os"
    "path/filepath"
    "testing"
)

func TestReadUTF8Text(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "hello.txt")
    if err := os.WriteFile(path, []byte("hello\nworld\n"), 0o644); err != nil {
        t.Fatalf("WriteFile() error = %v", err)
    }

    got, err := Read(path)
    if err != nil {
        t.Fatalf("Read() error = %v", err)
    }
    if got != "hello\nworld\n" {
        t.Fatalf("Read() = %q", got)
    }
}

func TestReadBinary(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "blob.bin")
    if err := os.WriteFile(path, []byte{0x00, 0x01, 0x02, 0x03}, 0o644); err != nil {
        t.Fatalf("WriteFile() error = %v", err)
    }

    _, err := Read(path)
    if err == nil {
        t.Fatal("expected binary file to be rejected")
    }
}
