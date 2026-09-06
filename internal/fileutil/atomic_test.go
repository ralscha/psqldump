package fileutil

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomicPreservesExistingFileOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact")
	if err := os.WriteFile(path, []byte("known good"), 0o600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	wantErr := errors.New("generation failed")
	err := WriteAtomic(path, 0o600, func(dst io.Writer) error {
		if _, err := dst.Write([]byte("partial")); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WriteAtomic error = %v, want %v", err, wantErr)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read existing file: %v", err)
	}
	if string(content) != "known good" {
		t.Fatalf("existing file was changed to %q", content)
	}
}

func TestWriteFileAtomicReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	if err := WriteFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic returned error: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replaced file: %v", err)
	}
	if string(content) != "new" {
		t.Fatalf("replaced file = %q, want new", content)
	}
}
