package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDoctor(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if ok := Doctor(s, &out); !ok {
		t.Errorf("healthy install failed doctor:\n%s", out.String())
	}

	// break it
	os.RemoveAll(filepath.Join(tmp, ".hydra", "skills", "skill-curator"))
	out.Reset()
	if ok := Doctor(s, &out); ok {
		t.Errorf("broken install passed doctor:\n%s", out.String())
	}
}
