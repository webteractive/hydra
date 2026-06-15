package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// makeTarGz packs content under the given entry name into a gzipped tar archive.
func makeTarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// updateServer spins up an httptest server mimicking the GitHub release layout.
// checksumOverride, when non-empty, is served instead of the real tarball sha256.
func updateServer(t *testing.T, tag, ver string, tarball []byte, checksumOverride string) *httptest.Server {
	t.Helper()
	file := fmt.Sprintf("hydra_%s_%s_%s.tar.gz", ver, runtime.GOOS, runtime.GOARCH)
	sum := fmt.Sprintf("%x", sha256.Sum256(tarball))
	if checksumOverride != "" {
		sum = checksumOverride
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"tag_name":%q}`, tag)
	})
	mux.HandleFunc("/download/"+tag+"/"+file, func(w http.ResponseWriter, _ *http.Request) {
		w.Write(tarball)
	})
	mux.HandleFunc("/download/"+tag+"/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", sum, file)
	})
	return httptest.NewServer(mux)
}

func TestSelfUpdateReplacesBinary(t *testing.T) {
	newBin := []byte("NEW-HYDRA-BINARY")
	tarball := makeTarGz(t, "hydra", newBin)
	srv := updateServer(t, "v9.9.9", "9.9.9", tarball, "")
	defer srv.Close()

	exePath := filepath.Join(t.TempDir(), "hydra")
	if err := os.WriteFile(exePath, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := SelfUpdate("0.1.0", exePath, runtime.GOOS, runtime.GOARCH,
		srv.URL+"/releases/latest", srv.URL+"/download", &buf)
	if err != nil {
		t.Fatalf("SelfUpdate: %v", err)
	}

	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, newBin) {
		t.Errorf("binary not replaced: got %q want %q", got, newBin)
	}
	fi, err := os.Stat(exePath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm()&0o100 == 0 {
		t.Errorf("binary not executable: mode %v", fi.Mode())
	}
	if !strings.Contains(buf.String(), "updated") {
		t.Errorf("output missing 'updated': %q", buf.String())
	}
}

func TestSelfUpdateAlreadyLatest(t *testing.T) {
	tarball := makeTarGz(t, "hydra", []byte("NEW-HYDRA-BINARY"))
	srv := updateServer(t, "v0.1.0", "0.1.0", tarball, "")
	defer srv.Close()

	exePath := filepath.Join(t.TempDir(), "hydra")
	if err := os.WriteFile(exePath, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := SelfUpdate("0.1.0", exePath, runtime.GOOS, runtime.GOARCH,
		srv.URL+"/releases/latest", srv.URL+"/download", &buf)
	if err != nil {
		t.Fatalf("SelfUpdate: %v", err)
	}

	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "OLD" {
		t.Errorf("binary should be unchanged: got %q", got)
	}
	if !strings.Contains(buf.String(), "already") {
		t.Errorf("output missing 'already': %q", buf.String())
	}
}

func TestSelfUpdateChecksumMismatch(t *testing.T) {
	tarball := makeTarGz(t, "hydra", []byte("NEW-HYDRA-BINARY"))
	srv := updateServer(t, "v9.9.9", "9.9.9", tarball, "deadbeef")
	defer srv.Close()

	exePath := filepath.Join(t.TempDir(), "hydra")
	if err := os.WriteFile(exePath, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := SelfUpdate("0.1.0", exePath, runtime.GOOS, runtime.GOARCH,
		srv.URL+"/releases/latest", srv.URL+"/download", &buf)
	if err == nil {
		t.Fatal("expected checksum mismatch error, got nil")
	}

	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "OLD" {
		t.Errorf("binary should be unchanged on mismatch: got %q", got)
	}
}
