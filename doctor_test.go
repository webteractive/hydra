package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctor(t *testing.T) {
	tmp := t.TempDir()
	s := ResolveScope(false, tmp, filepath.Join(tmp, "home"))
	var out bytes.Buffer
	if err := Init(s, &out); err != nil {
		t.Fatal(err)
	}

	if rep := Doctor(s); !rep.OK {
		var b bytes.Buffer
		renderDoctorText(rep, &b)
		t.Errorf("healthy install failed doctor:\n%s", b.String())
	}

	// renderDoctorText reproduces the legacy text format on a healthy install.
	var healthy bytes.Buffer
	renderDoctorText(Doctor(s), &healthy)
	if !strings.Contains(healthy.String(), "doctor: PASS") {
		t.Errorf("healthy text missing PASS:\n%s", healthy.String())
	}

	// break it
	os.RemoveAll(filepath.Join(tmp, ".hydra", "skills", "skill-curator"))
	if rep := Doctor(s); rep.OK {
		var b bytes.Buffer
		renderDoctorText(rep, &b)
		t.Errorf("broken install passed doctor:\n%s", b.String())
	}
}

func TestRunDoctorJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	t.Setenv("HOME", filepath.Join(tmp, "home"))

	if _, err := runCLI(t, "init"); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor --json: %v\n%s", err, out)
	}
	var rep DoctorReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if !rep.OK {
		t.Errorf("expected ok:true, got %+v", rep)
	}
}
