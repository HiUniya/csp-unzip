package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestRemoveWorkDirAfterWindowsSharingViolation(t *testing.T) {
	work := t.TempDir()
	path := filepath.Join(work, "7z.dll")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	// Keep the file readable and writable, but deny deletion while open.
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); !retryableCleanupError(err) {
		syscall.CloseHandle(handle)
		t.Fatalf("expected a retryable Windows removal error with the file open, got %v", err)
	}
	closed := make(chan error, 1)
	time.AfterFunc(100*time.Millisecond, func() {
		closed <- syscall.CloseHandle(handle)
	})
	removeErr := removeWorkDir(work)
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	if removeErr != nil {
		t.Fatalf("removeWorkDir did not recover after the handle closed: %v", removeErr)
	}
	if _, err := os.Lstat(work); !os.IsNotExist(err) {
		t.Fatalf("work directory still accessible: %v", err)
	}
}
