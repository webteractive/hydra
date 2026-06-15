package main

import (
	"os"
	"path/filepath"
	"strings"
)

func parseConfig(path string) map[string]string {
	m := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		m[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return m
}

func runtimes(s Scope) []string {
	r := parseConfig(filepath.Join(s.Home, "config"))["HYDRA_RUNTIMES"]
	if r == "" {
		r = "claude agents"
	}
	return strings.Fields(r)
}
