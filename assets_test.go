package main

import (
	"os"
	"strings"
	"testing"
)

func TestEmbeddedAssets(t *testing.T) {
	sc := string(assetBytes("skill-curator/SKILL.md"))
	if !strings.HasPrefix(sc, "---") || !strings.Contains(sc, "name: skill-curator") {
		t.Fatalf("skill-curator asset wrong:\n%s", sc[:min(80, len(sc))])
	}
	if !strings.Contains(string(assetBytes("curator-block.md")), "hydra:curator:start") {
		t.Fatal("curator-block missing start marker")
	}
	if !strings.Contains(string(assetBytes("config")), "HYDRA_RUNTIMES") {
		t.Fatal("config missing HYDRA_RUNTIMES")
	}
	if !strings.Contains(string(assetBytes("curator-reminder.sh")), "skill-curator") {
		t.Fatal("hook missing nudge text")
	}
	want, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatalf("read VERSION: %v", err)
	}
	if v := version(); v != strings.TrimSpace(string(want)) {
		t.Fatalf("version = %q, want %q", v, strings.TrimSpace(string(want)))
	}
}
