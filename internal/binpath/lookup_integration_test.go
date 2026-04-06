package binpath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLookup_findsInPath(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires exec.LookPath")
	}

	origDirs := wellKnownDirs
	t.Cleanup(func() { ResetForTesting(origDirs) })
	ResetForTesting(nil)

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
	if testing.Short() {
		t.Skip("integration test: requires file system")
	}

	origDirs := wellKnownDirs
	t.Cleanup(func() { ResetForTesting(origDirs) })

	tmpDir := t.TempDir()
	ResetForTesting([]string{tmpDir})

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
	if testing.Short() {
		t.Skip("integration test: requires exec.LookPath")
	}

	origDirs := wellKnownDirs
	t.Cleanup(func() { ResetForTesting(origDirs) })
	ResetForTesting(nil)

	got := Lookup("nonexistent-binary-xyz-12345")
	if got != "nonexistent-binary-xyz-12345" {
		t.Fatalf("expected bare name fallback, got %q", got)
	}
}

func TestLookup_caches(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires exec.LookPath")
	}

	origDirs := wellKnownDirs
	t.Cleanup(func() { ResetForTesting(origDirs) })
	ResetForTesting(nil)

	first := Lookup("ls")
	second := Lookup("ls")
	if first != second {
		t.Fatalf("cache inconsistency: %q != %q", first, second)
	}
}
