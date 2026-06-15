package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// httpClient bounds self-update network calls so a hung request can't hang the CLI.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// SelfUpdate replaces the binary at exePath with the latest GitHub release build
// for goos/goarch. URLs are injectable so tests can drive it with httptest.
//
//   - apiURL returns JSON {"tag_name": "..."} (the latest release tag).
//   - baseURL is the release-download root; assets live under baseURL/<tag>/.
//
// When the latest version matches curVersion it is a no-op (no download, no
// replace). The download is verified against checksums.txt before the running
// binary is atomically swapped via os.Rename within its own directory.
func SelfUpdate(curVersion, exePath, goos, goarch, apiURL, baseURL string, out io.Writer) error {
	tag, err := latestTag(apiURL)
	if err != nil {
		return err
	}
	ver := strings.TrimPrefix(tag, "v")

	if ver == curVersion {
		fmt.Fprintf(out, "already on the latest version (%s)\n", ver)
		return nil
	}

	file := fmt.Sprintf("hydra_%s_%s_%s.tar.gz", ver, goos, goarch)
	assetURL := baseURL + "/" + tag + "/" + file
	tarball, err := download(assetURL)
	if err != nil {
		return err
	}

	checksums, err := download(baseURL + "/" + tag + "/checksums.txt")
	if err != nil {
		return err
	}
	if err := verifyChecksum(tarball, file, checksums); err != nil {
		return err
	}

	bin, err := extractBinary(tarball, "hydra")
	if err != nil {
		return err
	}

	if err := replaceBinary(exePath, bin); err != nil {
		return err
	}

	fmt.Fprintf(out, "updated %s -> %s\n", curVersion, ver)
	return nil
}

func latestTag(apiURL string) (string, error) {
	body, err := download(apiURL)
	if err != nil {
		return "", err
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", fmt.Errorf("parse release metadata: %w", err)
	}
	if rel.TagName == "" {
		return "", fmt.Errorf("no tag_name in release metadata")
	}
	return rel.TagName, nil
}

func download(url string) ([]byte, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// verifyChecksum confirms tarball's sha256 matches the entry for file in a
// checksums.txt body (lines of "<sha256>  <filename>").
func verifyChecksum(tarball []byte, file string, checksums []byte) error {
	want := ""
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == file {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("no checksum entry for %s", file)
	}
	got := fmt.Sprintf("%x", sha256.Sum256(tarball))
	if got != want {
		return fmt.Errorf("checksum mismatch for %s: got %s want %s", file, got, want)
	}
	return nil
}

// extractBinary pulls the named entry out of a gzipped tar archive.
func extractBinary(tarball []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(strings.NewReader(string(tarball)))
	if err != nil {
		return nil, fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}
		if filepath.Base(hdr.Name) == name {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("%q not found in archive", name)
}

// replaceBinary atomically swaps the file at exePath with bin. The temp file is
// written in the same directory so os.Rename stays on one filesystem.
func replaceBinary(exePath string, bin []byte) error {
	dir := filepath.Dir(exePath)
	tmp, err := os.CreateTemp(dir, ".hydra-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { os.Remove(tmpName) }

	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, exePath); err != nil {
		cleanup()
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

func newSelfUpdateCmd(out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:     "self-update",
		Aliases: []string{"selfupdate"},
		Short:   "update hydra in place to the latest GitHub release",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			exe, err := os.Executable()
			if err != nil {
				return err
			}
			if r, e := filepath.EvalSymlinks(exe); e == nil {
				exe = r
			}
			return SelfUpdate(
				version(), exe, runtime.GOOS, runtime.GOARCH,
				"https://api.github.com/repos/webteractive/hydra/releases/latest",
				"https://github.com/webteractive/hydra/releases/download",
				cmd.OutOrStdout(),
			)
		},
	}
}
