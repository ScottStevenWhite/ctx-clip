package textfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadUTF16LEWithBOM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello-utf16.txt")
	data := []byte{0xFF, 0xFE, 'h', 0x00, 'i', 0x00, '\n', 0x00}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got != "hi\n" {
		t.Fatalf("Read() = %q, want %q", got, "hi\\n")
	}
}
