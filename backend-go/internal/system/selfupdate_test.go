package system

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTarGz(t *testing.T, entries []tar.Header, bodies map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, h := range entries {
		body := bodies[h.Name]
		h.Size = int64(len(body))
		if h.Mode == 0 {
			h.Mode = 0644
		}
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(t.TempDir(), "front.tar.gz")
	if err := os.WriteFile(name, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestExtractTarGzKeepsEntriesInside(t *testing.T) {
	good := writeTarGz(t, []tar.Header{
		{Name: "./", Typeflag: tar.TypeDir},
		{Name: "./assets/", Typeflag: tar.TypeDir},
		{Name: "./index.html", Typeflag: tar.TypeReg},
		{Name: "./assets/main.wasm.gz", Typeflag: tar.TypeReg},
	}, map[string]string{"./index.html": "<html>", "./assets/main.wasm.gz": "wasm"})
	dest := t.TempDir()
	if err := extractTarGz(good, dest); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dest, "assets", "main.wasm.gz")); err != nil || string(data) != "wasm" {
		t.Fatalf("extracted %q %v", data, err)
	}

	for name, header := range map[string]tar.Header{
		"traversal": {Name: "../escape", Typeflag: tar.TypeReg},
		"absolute":  {Name: "/etc/escape", Typeflag: tar.TypeReg},
		"symlink":   {Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"},
		"hardlink":  {Name: "hard", Typeflag: tar.TypeLink, Linkname: "/etc/passwd"},
	} {
		dest := t.TempDir()
		if err := extractTarGz(writeTarGz(t, []tar.Header{header}, nil), dest); err == nil {
			t.Errorf("%s: accepted", name)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(dest), "escape")); err == nil {
			t.Errorf("%s: wrote outside the destination", name)
		}
	}
}

func TestReleaseChecksums(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "gowave-linux-amd64")
	if err := os.WriteFile(bin, []byte("binary"), 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("binary"))
	sums := filepath.Join(dir, "SHA256SUMS")
	content := hex.EncodeToString(sum[:]) + "  gowave-linux-amd64\n" + strings.Repeat("0", 64) + " *gowave-frontend.tar.gz\n"
	if err := os.WriteFile(sums, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	parsed, err := readChecksums(sums)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(bin, parsed["gowave-linux-amd64"]); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(bin, parsed["gowave-frontend.tar.gz"]); err == nil {
		t.Fatal("accepted a mismatched checksum")
	}
	if err := verifyChecksum(bin, parsed["missing"]); err == nil {
		t.Fatal("accepted a file missing from SHA256SUMS")
	}
}

func TestRootOnlyWritableRejectsServiceAccountFiles(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("files created by root are trusted")
	}
	file := filepath.Join(t.TempDir(), "gowave-server")
	if err := os.WriteFile(file, nil, 0755); err != nil {
		t.Fatal(err)
	}
	if err := rootOnlyWritable(file, false); err == nil {
		t.Fatal("trusted a binary owned by an unprivileged user")
	}
	if err := ApplyRelease(t.TempDir()); err == nil || !strings.Contains(err.Error(), "root") {
		t.Fatalf("ApplyRelease as non-root: %v", err)
	}
}

func TestApplyUpdateRequestsRootHelper(t *testing.T) {
	t.Setenv("IN_DOCKER", "")
	t.Setenv("DOCKER", "")
	t.Setenv("CONTAINER", "")
	if IsRunningInDocker() {
		t.Skip("running inside a container")
	}
	dir := t.TempDir()
	oldUnit, oldDir := updateHelperUnit, updateRequestDir
	t.Cleanup(func() { updateHelperUnit, updateRequestDir = oldUnit, oldDir })
	updateHelperUnit, updateRequestDir = filepath.Join(dir, "gowave-update.path"), dir
	t.Setenv("RUNTIME_DIRECTORY", "")

	w := httptest.NewRecorder()
	(&Handler{}).ApplyUpdate(w, httptest.NewRequest(http.MethodPost, "/api/system/updates/apply", nil))
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "install.sh") {
		t.Fatalf("without helper: %d %s", w.Code, w.Body.String())
	}

	if err := os.WriteFile(updateHelperUnit, nil, 0644); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	(&Handler{}).ApplyUpdate(w, httptest.NewRequest(http.MethodPost, "/api/system/updates/apply", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("with helper: %d %s", w.Code, w.Body.String())
	}
	if info, err := os.Stat(filepath.Join(dir, updateRequestName)); err != nil || info.Size() != 0 {
		t.Fatalf("update request: %v %v", info, err)
	}
}
