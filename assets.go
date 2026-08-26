package main

import (
	_ "embed"
	"os"
	"strings"
)

//go:embed VERSION
var versionData string

// injectedVersion is set at release time via -ldflags "-X main.injectedVersion=<tag>"
// (see .goreleaser.yaml). When empty (dev/`go build`), the embedded VERSION is used.
var injectedVersion string

func version() string {
	if injectedVersion != "" {
		return injectedVersion
	}
	return strings.TrimSpace(versionData)
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

func isDir(p string) bool { fi, err := os.Stat(p); return err == nil && fi.IsDir() }

func fileContains(p, sub string) bool {
	b, err := os.ReadFile(p)
	return err == nil && strings.Contains(string(b), sub)
}
