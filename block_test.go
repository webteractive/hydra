package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSpliceBlockCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "CLAUDE.md")
	block := blockStart + "\nhello\n" + blockEnd + "\n"
	if err := SpliceBlock(path, block); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != block {
		t.Errorf("content = %q want %q", got, block)
	}
}

func TestSpliceBlockAppendsPreservingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := os.WriteFile(path, []byte("# Project\n\nExisting guidance.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	block := blockStart + "\nhello\n" + blockEnd + "\n"
	if err := SpliceBlock(path, block); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, path)
	if !strings.HasPrefix(got, "# Project\n\nExisting guidance.\n") {
		t.Errorf("existing content lost: %q", got)
	}
	if !strings.Contains(got, "hello") {
		t.Errorf("block not appended: %q", got)
	}
}

func TestSpliceBlockReplacesInPlace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	original := "# Project\n\n" + blockStart + "\nold\n" + blockEnd + "\n\n## After\n\nTrailing guidance.\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SpliceBlock(path, blockStart+"\nnew\n"+blockEnd+"\n"); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, path)
	if strings.Contains(got, "old") {
		t.Errorf("old block survived: %q", got)
	}
	if !strings.Contains(got, "new") {
		t.Errorf("new block missing: %q", got)
	}
	if !strings.Contains(got, "## After") || !strings.Contains(got, "# Project") {
		t.Errorf("surrounding content disturbed: %q", got)
	}
	if strings.Count(got, blockStart) != 1 {
		t.Errorf("expected exactly one block, got %d: %q", strings.Count(got, blockStart), got)
	}
}

func TestSpliceBlockIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	block := blockStart + "\nhello\n" + blockEnd + "\n"
	for i := 0; i < 3; i++ {
		if err := SpliceBlock(path, block); err != nil {
			t.Fatal(err)
		}
	}
	if got := readFile(t, path); strings.Count(got, blockStart) != 1 {
		t.Errorf("repeated splices duplicated the block: %q", got)
	}
}

func TestStripBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	content := "# Project\n\n<!-- hydra:curator:start -->\ncurator\n<!-- hydra:curator:end -->\n\n## After\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := StripBlock(path, "<!-- hydra:curator:start -->", "<!-- hydra:curator:end -->")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Error("found = false, want true")
	}
	got := readFile(t, path)
	if strings.Contains(got, "curator") {
		t.Errorf("block survived: %q", got)
	}
	if !strings.Contains(got, "# Project") || !strings.Contains(got, "## After") {
		t.Errorf("surrounding content lost: %q", got)
	}
}

func TestStripBlockAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := os.WriteFile(path, []byte("# Project\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := StripBlock(path, "<!-- hydra:curator:start -->", "<!-- hydra:curator:end -->")
	if err != nil || found {
		t.Errorf("found = %v err = %v, want false nil", found, err)
	}
}

func TestStripBlockMissingFile(t *testing.T) {
	found, err := StripBlock(filepath.Join(t.TempDir(), "nope.md"), blockStart, blockEnd)
	if err != nil || found {
		t.Errorf("found = %v err = %v, want false nil", found, err)
	}
}
