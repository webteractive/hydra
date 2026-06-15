package main

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:assets
var assetsFS embed.FS

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

// assetBytes returns an embedded asset by its path under assets/ (e.g. "config").
func assetBytes(name string) []byte {
	b, err := assetsFS.ReadFile("assets/" + name)
	if err != nil {
		panic("missing embedded asset: " + name)
	}
	return b
}

func writeAsset(dst, name string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, assetBytes(name), mode)
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

func isDir(p string) bool { fi, err := os.Stat(p); return err == nil && fi.IsDir() }

// resolves reports whether p is a symlink whose target exists.
func resolves(p string) bool {
	fi, err := os.Lstat(p)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

func fileContains(p, sub string) bool {
	b, err := os.ReadFile(p)
	return err == nil && strings.Contains(string(b), sub)
}
