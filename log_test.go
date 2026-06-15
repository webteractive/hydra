package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLog(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}

	if err := Log(s, "CREATE", "my-skill", "first version", &out, "2026-06-15"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(tmp, ".hydra", "curator.log"))
	if want := "2026-06-15  CREATE  my-skill  — first version\n"; !contains(string(b), want) {
		t.Errorf("log missing line; got:\n%s", b)
	}

	if err := Log(s, "NOPE", "my-skill", "reason", &out, "2026-06-15"); err == nil {
		t.Error("expected error for invalid ACTION")
	}
	if err := Log(s, "UPDATE", "my-skill", "", &out, "2026-06-15"); err == nil {
		t.Error("expected error for empty reason")
	}
}

func contains(s, sub string) bool { return bytes.Contains([]byte(s), []byte(sub)) }
