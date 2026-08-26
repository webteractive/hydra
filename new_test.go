package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewScaffoldsRule(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))

	var out bytes.Buffer
	if err := New(s, "rust-dependencies", &out); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tmp, ".hydra", "rules", "rust-dependencies.md")
	got := readFile(t, path)
	for _, want := range []string{"paths:", "commands:", "triggers:", "# Rust Dependencies"} {
		if !strings.Contains(got, want) {
			t.Errorf("scaffold missing %q:\n%s", want, got)
		}
	}
}

func TestNewRejectsBadNames(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	for _, name := range []string{"", "Not Kebab", "UPPER", "trailing-"} {
		if err := New(s, name, &out); err == nil {
			t.Errorf("expected an error for %q", name)
		}
	}
}

func TestNewRefusesToClobber(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := New(s, "dupe", &out); err != nil {
		t.Fatal(err)
	}
	if err := New(s, "dupe", &out); err == nil {
		t.Error("expected an error for an existing rule")
	}
}
