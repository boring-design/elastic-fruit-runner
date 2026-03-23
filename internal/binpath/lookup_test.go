package binpath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLookup_findsInPath(t *testing.T) {
	// "ls" should always be found on any Unix system
	got := Lookup("ls")
	if got == "ls" {
		t.Fatal("expected absolute path for ls, got bare name")
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("expected absolute path, got %q", got)
	}
}

func TestLookup_fallsBackToWellKnownDirs(t *testing.T) {
	// Create a temp binary in a well-known dir simulation
	tmpDir := t.TempDir()

	// Override wellKnownDirs for this test
	origDirs := wellKnownDirs
	wellKnownDirs = []string{tmpDir}
	t.Cleanup(func() { wellKnownDirs = origDirs })

	// Clear cache
	cacheMu.Lock()
	delete(cache, "test-fake-bin")
	cacheMu.Unlock()

	fakeBin := filepath.Join(tmpDir, "test-fake-bin")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := Lookup("test-fake-bin")
	if got != fakeBin {
		t.Fatalf("expected %q, got %q", fakeBin, got)
	}
}

func TestLookup_returnsBareName_whenNotFound(t *testing.T) {
	cacheMu.Lock()
	delete(cache, "nonexistent-binary-xyz-12345")
	cacheMu.Unlock()

	got := Lookup("nonexistent-binary-xyz-12345")
	if got != "nonexistent-binary-xyz-12345" {
		t.Fatalf("expected bare name fallback, got %q", got)
	}
}

func TestLookup_caches(t *testing.T) {
	cacheMu.Lock()
	delete(cache, "ls")
	cacheMu.Unlock()

	first := Lookup("ls")
	second := Lookup("ls")
	if first != second {
		t.Fatalf("cache inconsistency: %q != %q", first, second)
	}
}
