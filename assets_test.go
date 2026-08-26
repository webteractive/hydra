package main

import (
	"os"
	"strings"
	"testing"
)

func TestVersionMatchesVersionFile(t *testing.T) {
	raw, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(raw))
	if got := version(); got != want {
		t.Errorf("version() = %q, want %q", got, want)
	}
}

func TestInjectedVersionWins(t *testing.T) {
	old := injectedVersion
	t.Cleanup(func() { injectedVersion = old })
	injectedVersion = "v9.9.9"
	if got := version(); got != "v9.9.9" {
		t.Errorf("version() = %q, want v9.9.9", got)
	}
}
