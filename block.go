package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var multiNewline = regexp.MustCompile(`\n{3,}`)

// SpliceBlock writes block into path between its sentinels. An existing block is
// replaced in place so re-running never appends a second copy and never disturbs
// what the user wrote around it; otherwise the block is appended. The file and
// its parent directory are created if missing.
func SpliceBlock(path, block string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	mode := os.FileMode(0o644)
	var content string
	if data, err := os.ReadFile(path); err == nil {
		content = string(data)
		if fi, err := os.Stat(path); err == nil {
			mode = fi.Mode().Perm()
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	block = strings.TrimRight(block, "\n") + "\n"

	var updated string
	if start := strings.Index(content, blockStart); start != -1 {
		if end := strings.Index(content[start:], blockEnd); end != -1 {
			tail := content[start+end+len(blockEnd):]
			updated = content[:start] + block + strings.TrimLeft(tail, "\n")
		}
	}
	if updated == "" {
		head := strings.TrimRight(content, "\n")
		if head == "" {
			updated = block
		} else {
			updated = head + "\n\n" + block
		}
	}

	updated = multiNewline.ReplaceAllString(updated, "\n\n")
	if !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	return os.WriteFile(path, []byte(updated), mode)
}

// StripBlock removes a sentinel-delimited block from path, reporting whether one
// was there. A missing file is not an error — there is simply nothing to strip.
func StripBlock(path, start, end string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	content := string(data)

	i := strings.Index(content, start)
	if i == -1 {
		return false, nil
	}
	j := strings.Index(content[i:], end)
	if j == -1 {
		return false, nil
	}

	updated := content[:i] + strings.TrimLeft(content[i+j+len(end):], "\n")
	updated = multiNewline.ReplaceAllString(updated, "\n\n")
	if strings.TrimSpace(updated) != "" && !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}

	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	return true, os.WriteFile(path, []byte(updated), mode)
}
