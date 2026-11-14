package system

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	request := filepath.Join(dir, updateRequestName)
	if got := readRequestedChannel(request); got != CurrentChannel() {
		t.Fatalf("default request channel = %q, want %q", got, CurrentChannel())
	}

	w = httptest.NewRecorder()
	(&Handler{}).ApplyUpdate(w, httptest.NewRequest(http.MethodPost, "/api/system/updates/apply", strings.NewReader(`{"channel":"dev"}`)))
	if w.Code != http.StatusOK || readRequestedChannel(request) != ChannelDev {
		t.Fatalf("switch to dev: %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	(&Handler{}).ApplyUpdate(w, httptest.NewRequest(http.MethodPost, "/api/system/updates/apply", strings.NewReader(`{"channel":"nightly"}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown channel: %d %s", w.Code, w.Body.String())
	}
}

func TestReadRequestedChannelRejectsUntrustedRequests(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"empty": "", "unknown": "nightly", "long": strings.Repeat("dev", 20)} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if got := readRequestedChannel(path); got != CurrentChannel() {
			t.Errorf("%s: channel %q", name, got)
		}
	}
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte(ChannelDev), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if got := readRequestedChannel(link); got != CurrentChannel() {
		t.Errorf("followed a symlinked request: %q", got)
	}
	if got := readRequestedChannel(target); got != ChannelDev {
		t.Errorf("valid request: %q", got)
	}
	if got := releaseDownloadBase(ChannelDev); !strings.HasSuffix(got, "/releases/download/dev/") {
		t.Errorf("dev download base %q", got)
	}
	if got := releaseDownloadBase(ChannelStable); !strings.HasSuffix(got, "/releases/latest/download/") {
		t.Errorf("stable download base %q", got)
	}
}

func TestInstallerCommandPassesChannelToScript(t *testing.T) {
	want := `sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/aaaSaZaN/gowave/main/install.sh)" _ --update --dev`
	if got := installerCommand(ChannelDev); got != want {
		t.Fatalf("installerCommand(dev) = %q, want %q", got, want)
	}
}

func useUpdateRuntimeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := updateRequestDir
	t.Cleanup(func() { updateRequestDir = old })
	updateRequestDir = dir
	t.Setenv("RUNTIME_DIRECTORY", "")
	return dir
}

func TestUpdateReporterWritesProgressAndErrors(t *testing.T) {
	dir := useUpdateRuntimeDir(t)

	report := newUpdateReporter(dir, ChannelDev)
	report.Step(UpdateStepDownload, 5, "Downloading %s", "gowave-linux-amd64")
	report.Download("gowave-linux-amd64", 50, 100, 5, 55)
	report.Fail(errors.New("download gowave-linux-amd64: bad status: 404 Not Found"))

	status, err := readUpdateStatus()
	if err != nil || status == nil {
		t.Fatalf("readUpdateStatus() = %v, %v", status, err)
	}
	if status.State != UpdateStateFailed || status.Step != UpdateStepDownload || status.Channel != ChannelDev {
		t.Fatalf("status = %+v", status)
	}
	if !strings.Contains(status.Error, "404") || status.FinishedAt == nil || status.Progress < 5 {
		t.Fatalf("failed status = %+v", status)
	}
	last := status.Log[len(status.Log)-1]
	if last.Level != "error" || !strings.Contains(last.Message, "404") {
		t.Fatalf("last log line = %+v", last)
	}
	if info, err := os.Stat(filepath.Join(dir, updateStatusName)); err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("status file mode: %v %v", info, err)
	}

	report = newUpdateReporter(dir, ChannelStable)
	report.Done("v9.9.9")
	status, err = readUpdateStatus()
	if err != nil || status.State != UpdateStateDone || status.Progress != 100 || status.InstalledVersion != "v9.9.9" {
		t.Fatalf("done status = %+v, %v", status, err)
	}
}

func TestUpdateStatusIgnoresLinks(t *testing.T) {
	dir := useUpdateRuntimeDir(t)
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, []byte(`{"state":"done"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, updateStatusName)); err != nil {
		t.Fatal(err)
	}
	if status, err := readUpdateStatus(); err == nil || status != nil {
		t.Fatalf("readUpdateStatus() through a link = %v, %v", status, err)
	}
	// The helper replaces the link with its own file and leaves the target alone.
	newUpdateReporter(dir, ChannelStable).Info("hello")
	if data, _ := os.ReadFile(target); string(data) != `{"state":"done"}` {
		t.Fatalf("link target changed: %s", data)
	}
	if status, err := readUpdateStatus(); err != nil || status.State != UpdateStateRunning {
		t.Fatalf("status after replace = %v, %v", status, err)
	}
}

func TestUpdateReporterKeepsLogBounded(t *testing.T) {
	dir := useUpdateRuntimeDir(t)
	report := newUpdateReporter(dir, ChannelStable)
	for i := 0; i < maxUpdateStatusLog+20; i++ {
		report.Info("line %d", i)
	}
	status, err := readUpdateStatus()
	if err != nil || len(status.Log) != maxUpdateStatusLog || status.Log[0].Message != "line 20" {
		t.Fatalf("log = %d lines, first %+v, %v", len(status.Log), status.Log[0], err)
	}
}

func TestUpdateStatusHandler(t *testing.T) {
	dir := useUpdateRuntimeDir(t)
	requestDir := dir

	get := func() map[string]interface{} {
		t.Helper()
		w := httptest.NewRecorder()
		(&Handler{}).UpdateStatus(w, httptest.NewRequest(http.MethodGet, "/api/system/updates/status", nil))
		if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
			t.Fatalf("status: %d %v", w.Code, w.Header())
		}
		var body map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body
	}

	body := get()
	if body["status"] != nil || body["requestPending"] != false {
		t.Fatalf("idle status = %v", body)
	}

	if err := os.WriteFile(filepath.Join(requestDir, updateRequestName), []byte("dev"), 0o600); err != nil {
		t.Fatal(err)
	}
	newUpdateReporter(dir, ChannelDev).Step(UpdateStepVerify, 76, "Checking SHA256SUMS")
	body = get()
	status, _ := body["status"].(map[string]interface{})
	if body["requestPending"] != true || status["step"] != UpdateStepVerify || status["state"] != UpdateStateRunning {
		t.Fatalf("running status = %v", body)
	}
}

// githubStub answers release API calls in place of api.github.com.
type githubStub struct {
	calls   int
	handler http.HandlerFunc
}

func useGitHubStub(t *testing.T, handler http.HandlerFunc) *githubStub {
	t.Helper()
	stub := &githubStub{handler: handler}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.calls++
		stub.handler(w, r)
	}))
	t.Cleanup(server.Close)
	target, _ := url.Parse(server.URL)
	oldClient := releaseHTTPClient
	releaseCache.Lock()
	releaseCache.byChannel = map[string]cachedRelease{}
	releaseCache.Unlock()
	t.Cleanup(func() {
		releaseHTTPClient = oldClient
		releaseCache.Lock()
		releaseCache.byChannel = map[string]cachedRelease{}
		releaseCache.Unlock()
	})
	releaseHTTPClient = &http.Client{Timeout: 5 * time.Second, Transport: rewriteTransport{target: target}}
	return stub
}

type rewriteTransport struct{ target *url.URL }

func (rt rewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.URL.Scheme, r.URL.Host = rt.target.Scheme, rt.target.Host
	return http.DefaultTransport.RoundTrip(r)
}

func checkUpdate(t *testing.T, query string) (int, map[string]interface{}) {
	t.Helper()
	w := httptest.NewRecorder()
	(&Handler{}).CheckUpdate(w, httptest.NewRequest(http.MethodGet, "/api/system/updates/check"+query, nil))
	if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		t.Fatalf("check is cacheable: %v", w.Header())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	return w.Code, body
}

func TestCheckUpdateCachesAndForces(t *testing.T) {
	name := "v99.0.0"
	stub := useGitHubStub(t, func(w http.ResponseWriter, r *http.Request) {
		etag := `"` + name + `"`
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		_ = json.NewEncoder(w).Encode(GitHubRelease{TagName: name, Name: name})
	})

	code, body := checkUpdate(t, "?channel=stable")
	if code != http.StatusOK || body["latestVersion"] != "v99.0.0" || stub.calls != 1 {
		t.Fatalf("first check: %d %v calls=%d", code, body, stub.calls)
	}
	name = "v99.0.1"
	if _, body = checkUpdate(t, "?channel=stable"); body["latestVersion"] != "v99.0.0" || stub.calls != 1 {
		t.Fatalf("cached check: %v calls=%d", body, stub.calls)
	}

	// A forced check right after a fetch is still answered from the cache.
	if _, body = checkUpdate(t, "?channel=stable&force=1"); stub.calls != 1 {
		t.Fatalf("forced check within %s: %v calls=%d", releaseForceInterval, body, stub.calls)
	}
	releaseCache.Lock()
	c := releaseCache.byChannel[ChannelStable]
	c.fetched = c.fetched.Add(-releaseForceInterval)
	releaseCache.byChannel[ChannelStable] = c
	releaseCache.Unlock()
	if _, body = checkUpdate(t, "?channel=stable&force=1"); body["latestVersion"] != "v99.0.1" || stub.calls != 2 {
		t.Fatalf("forced check: %v calls=%d", body, stub.calls)
	}

	// An unchanged release is answered with 304 and keeps the cached one.
	releaseCache.Lock()
	c = releaseCache.byChannel[ChannelStable]
	c.fetched = c.fetched.Add(-releaseCacheTTL)
	releaseCache.byChannel[ChannelStable] = c
	releaseCache.Unlock()
	if _, body = checkUpdate(t, "?channel=stable"); body["latestVersion"] != "v99.0.1" || stub.calls != 3 {
		t.Fatalf("not modified: %v calls=%d", body, stub.calls)
	}
}

func TestCheckUpdateInDockerStaysOnCurrentChannel(t *testing.T) {
	t.Setenv("IN_DOCKER", "true")
	t.Setenv("GOWAVE_COMPOSE_DIR", "/opt/gowave-docker")
	useGitHubStub(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(GitHubRelease{TagName: "v99.0.0", Name: "v99.0.0"})
	})

	code, body := checkUpdate(t, "?channel=dev")
	if code != http.StatusOK || body["channel"] != CurrentChannel() || body["targetChannel"] != CurrentChannel() {
		t.Fatalf("check in Docker = %d %v, want the current channel", code, body)
	}
	if got, want := body["updateCommand"], "cd /opt/gowave-docker && docker compose pull && docker compose up -d"; got != want {
		t.Errorf("updateCommand = %v, want %q", got, want)
	}
}

func TestCheckUpdateExplainsRateLimit(t *testing.T) {
	stub := useGitHubStub(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1800000000")
		w.WriteHeader(http.StatusForbidden)
	})
	code, body := checkUpdate(t, "?channel=stable")
	message, _ := body["message"].(string)
	if code != http.StatusBadGateway || !strings.Contains(message, "rate limit") || !strings.Contains(message, "UTC") {
		t.Fatalf("rate limited: %d %v", code, body)
	}
	// The failure is remembered briefly instead of asking GitHub on every check.
	checkUpdate(t, "?channel=stable")
	if stub.calls != 1 {
		t.Fatalf("calls after a failure = %d, want 1", stub.calls)
	}
}
