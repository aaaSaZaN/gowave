package system

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Limits for a release download; the real artifacts are far smaller.
const (
	maxReleaseFile     = 256 << 20
	maxFrontendContent = 512 << 20
)

// ApplyRelease is the root side of an in-panel update (`gowave update apply`,
// started by gowave-update.service). It installs the newest build of the
// requested channel (stable or dev) after checking SHA256SUMS, keeps the code
// root-owned, reapplies the service policy and restarts the panel. Every step is
// printed to the journal and reported to the panel through the status file.
func ApplyRelease(installDir string) error {
	if os.Geteuid() != 0 {
		return errors.New("update apply must run as root")
	}
	// The request only names the channel; it has served its purpose once read.
	request := filepath.Join(updateRequestDir, updateRequestName)
	channel := readRequestedChannel(request)
	_ = os.Remove(request)

	report := newUpdateReporter(updateRequestDir, channel)
	if err := applyRelease(installDir, channel, report); err != nil {
		report.Fail(err)
		return err
	}
	return nil
}

// Download progress shares of the overall progress: the binary is the largest file.
var releaseFiles = []struct {
	name     string
	from, to int
}{
	{"SHA256SUMS", 2, 5},
	{"", 5, 55}, // the binary for this architecture
	{"gowave-frontend.tar.gz", 55, 75},
}

func applyRelease(installDir, channel string, report *updateReporter) error {
	report.Step(UpdateStepStarting, 1, "Installing the latest %s build (running %s)", channel, AppVersion)
	installDir = filepath.Clean(installDir)
	exe, err := trustedExecutable(installDir)
	if err != nil {
		return err
	}
	baseURL := releaseDownloadBase(channel)

	work, err := os.MkdirTemp("", "gowave-update-")
	if err != nil {
		return fmt.Errorf("create a temporary directory: %w", err)
	}
	defer os.RemoveAll(work)

	binName := "gowave-linux-" + runtime.GOARCH
	const frontName = "gowave-frontend.tar.gz"
	client := &http.Client{Timeout: 10 * time.Minute}
	for _, file := range releaseFiles {
		name := file.name
		if name == "" {
			name = binName
		}
		from, to := file.from, file.to
		report.Step(UpdateStepDownload, from, "Downloading %s", baseURL+name)
		size, err := download(client, baseURL+name, filepath.Join(work, name), func(downloaded, total int64) {
			report.Download(name, downloaded, total, from, to)
		})
		if err != nil {
			return fmt.Errorf("download %s: %w", baseURL+name, err)
		}
		report.Info("Downloaded %s (%s)", name, formatPrettyBytes(size))
	}

	report.Step(UpdateStepVerify, 76, "Checking SHA256SUMS")
	sums, err := readChecksums(filepath.Join(work, "SHA256SUMS"))
	if err != nil {
		return fmt.Errorf("read SHA256SUMS: %w", err)
	}
	for _, name := range []string{binName, frontName} {
		if err := verifyChecksum(filepath.Join(work, name), sums[name]); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	installed := releaseBinaryVersion(filepath.Join(work, binName))
	if installed != "" {
		report.Info("Checksums match, downloaded build is %s", installed)
	} else {
		report.Info("Checksums match")
	}

	report.Step(UpdateStepFrontend, 80, "Installing the frontend into %s", filepath.Join(installDir, "frontend", "dist"))
	if err := installFrontend(filepath.Join(work, frontName), filepath.Join(installDir, "frontend")); err != nil {
		return fmt.Errorf("install frontend: %w", err)
	}
	report.Step(UpdateStepBinary, 86, "Replacing %s (previous build kept as %s.bak)", exe, filepath.Base(exe))
	if err := installBinary(filepath.Join(work, binName), exe); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}

	report.Step(UpdateStepService, 90, "Reapplying the service policy")
	if out, err := runCommand(exe, "service", "install", installDir); err != nil {
		return commandError("service install", out, err)
	}
	report.Step(UpdateStepRestart, 95, "Restarting gowave.service")
	if out, err := runCommand("systemctl", "restart", "gowave.service"); err != nil {
		return commandError("systemctl restart gowave.service", out, err)
	}
	report.Done(installed)
	return nil
}

// releaseBinaryVersion asks a downloaded, verified binary for its version.
func releaseBinaryVersion(name string) string {
	if err := os.Chmod(name, 0o700); err != nil {
		return ""
	}
	out, err := exec.Command(name, "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// runCommand mirrors the command's output to the journal and returns it for the report.
func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var buf strings.Builder
	cmd.Stdout = io.MultiWriter(os.Stdout, &limitedBuilder{b: &buf, max: 4 << 10})
	cmd.Stderr = io.MultiWriter(os.Stderr, &limitedBuilder{b: &buf, max: 4 << 10})
	err := cmd.Run()
	return strings.TrimSpace(buf.String()), err
}

func commandError(what, output string, err error) error {
	if output != "" {
		return fmt.Errorf("%s: %w: %s", what, err, output)
	}
	return fmt.Errorf("%s: %w", what, err)
}

type limitedBuilder struct {
	b   *strings.Builder
	max int
}

func (l *limitedBuilder) Write(p []byte) (int, error) {
	if room := l.max - l.b.Len(); room > 0 {
		if len(p) > room {
			l.b.Write(p[:room])
		} else {
			l.b.Write(p)
		}
	}
	return len(p), nil
}

// trustedExecutable refuses to act as root unless the running binary and its
// directory cannot be modified by the unprivileged service account.
func trustedExecutable(installDir string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return "", err
	}
	if want := filepath.Join(installDir, "gowave-server"); exe != want {
		return "", fmt.Errorf("refusing to update: running %s, expected %s", exe, want)
	}
	if err := rootOnlyWritable(exe, false); err != nil {
		return "", err
	}
	if err := rootOnlyWritable(installDir, true); err != nil {
		return "", err
	}
	return exe, nil
}

// rootOnlyWritable reports an error unless name is owned by root and nobody else
// can replace it. A shared directory is accepted only with the sticky bit set,
// which stops other users from renaming or removing root's files in it.
func rootOnlyWritable(name string, isDir bool) error {
	info, err := os.Lstat(name)
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Uid != 0 || info.IsDir() != isDir {
		return fmt.Errorf("refusing to update: %s must be a root-owned %s", name, map[bool]string{true: "directory", false: "file"}[isDir])
	}
	if info.Mode().Perm()&0o022 != 0 && !(isDir && info.Mode()&fs.ModeSticky != 0) {
		return fmt.Errorf("refusing to update: %s is writable by other users", name)
	}
	return nil
}

func download(client *http.Client, url, dest string, progress func(downloaded, total int64)) (int64, error) {
	resp, err := client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("bad status: %s", resp.Status)
	}
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return 0, err
	}
	var w io.Writer = out
	if progress != nil {
		total := resp.ContentLength
		w = &progressWriter{w: out, report: func(n int64) { progress(n, total) }}
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, maxReleaseFile+1))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil && n > maxReleaseFile {
		err = errors.New("file is too large")
	}
	if err == nil && resp.ContentLength > 0 && n != resp.ContentLength {
		err = fmt.Errorf("received %d of %d bytes", n, resp.ContentLength)
	}
	return n, err
}

// readChecksums parses `sha256sum` output: "<hex>  <name>" or "<hex> *<name>".
func readChecksums(name string) (map[string]string, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sums := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && len(fields[0]) == sha256.Size*2 {
			sums[strings.TrimPrefix(fields[1], "*")] = strings.ToLower(fields[0])
		}
	}
	return sums, scanner.Err()
}

func verifyChecksum(name, want string) error {
	if want == "" {
		return errors.New("not listed in SHA256SUMS")
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("checksum %s does not match SHA256SUMS %s", got, want)
	}
	return nil
}

// installFrontend unpacks the archive next to the live directory and swaps it in.
func installFrontend(archive, frontendDir string) error {
	if err := os.MkdirAll(frontendDir, 0o755); err != nil {
		return err
	}
	if err := rootOnlyWritable(frontendDir, true); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(frontendDir, "dist.new-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := os.Chmod(staging, 0o755); err != nil {
		return err
	}
	if err := extractTarGz(archive, staging); err != nil {
		return err
	}
	live, old := filepath.Join(frontendDir, "dist"), filepath.Join(frontendDir, "dist.old")
	_ = os.RemoveAll(old)
	if err := os.Rename(live, old); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(staging, live); err != nil {
		_ = os.Rename(old, live)
		return err
	}
	return os.RemoveAll(old)
}

// extractTarGz accepts only directories and regular files that stay inside dest.
func extractTarGz(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var total int64
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := path.Clean(header.Name)
		if name == "." {
			continue
		}
		if path.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") {
			return fmt.Errorf("unsafe path %q in archive", header.Name)
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if total += header.Size; total > maxFrontendContent {
				return errors.New("archive is too large")
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if err != nil {
				return err
			}
			_, err = io.CopyN(out, tr, header.Size)
			if closeErr := out.Close(); err == nil {
				err = closeErr
			}
			if err != nil {
				return err
			}
			_ = os.Chtimes(target, header.ModTime, header.ModTime)
		default:
			return fmt.Errorf("unsupported entry %q in archive", header.Name)
		}
	}
}

// installBinary replaces exe atomically and keeps the previous build as .bak.
func installBinary(src, exe string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(exe), ".gowave-server-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	_ = os.Remove(exe + ".bak")
	if err := os.Rename(exe, exe+".bak"); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), exe); err != nil {
		_ = os.Rename(exe+".bak", exe)
		return err
	}
	return nil
}

// The root update helper reports its progress in update-status.json next to the
// update request. The directory belongs to the panel, so root only creates a new
// file there (O_EXCL never follows links) and renames it over the old status.
const (
	updateStatusName    = "update-status.json"
	maxUpdateStatusSize = 256 << 10
	maxUpdateStatusLog  = 200
)

// Update states and steps reported to the panel.
const (
	UpdateStateRunning = "running"
	UpdateStateDone    = "done"
	UpdateStateFailed  = "failed"

	UpdateStepStarting = "starting"
	UpdateStepDownload = "download"
	UpdateStepVerify   = "verify"
	UpdateStepFrontend = "frontend"
	UpdateStepBinary   = "binary"
	UpdateStepService  = "service"
	UpdateStepRestart  = "restart"
	UpdateStepDone     = "done"
)

type UpdateLogLine struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// UpdateStatus is the progress of the latest in-panel update.
type UpdateStatus struct {
	State   string `json:"state"`
	Step    string `json:"step"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
	// Progress is the overall progress in percent.
	Progress int `json:"progress"`
	// Downloaded and Total describe the file being downloaded, in bytes.
	Downloaded int64  `json:"downloaded,omitempty"`
	Total      int64  `json:"total,omitempty"`
	File       string `json:"file,omitempty"`

	Channel          string          `json:"channel"`
	FromVersion      string          `json:"fromVersion"`
	InstalledVersion string          `json:"installedVersion,omitempty"`
	StartedAt        time.Time       `json:"startedAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
	FinishedAt       *time.Time      `json:"finishedAt,omitempty"`
	Log              []UpdateLogLine `json:"log"`
}

// updateReporter keeps the status file current while the root helper works.
// Failing to write the status never stops the update; it is only logged.
type updateReporter struct {
	mu        sync.Mutex
	dir       string
	status    UpdateStatus
	lastWrite time.Time
	warned    bool
}

func newUpdateReporter(dir, channel string) *updateReporter {
	now := time.Now().UTC()
	r := &updateReporter{dir: dir, status: UpdateStatus{
		State:       UpdateStateRunning,
		Step:        UpdateStepStarting,
		Channel:     channel,
		FromVersion: AppVersion,
		StartedAt:   now,
		UpdatedAt:   now,
		Log:         []UpdateLogLine{},
	}}
	return r
}

func (r *updateReporter) addLog(level, message string) {
	r.status.Log = append(r.status.Log, UpdateLogLine{Time: time.Now().UTC(), Level: level, Message: message})
	if extra := len(r.status.Log) - maxUpdateStatusLog; extra > 0 {
		r.status.Log = append([]UpdateLogLine(nil), r.status.Log[extra:]...)
	}
}

// Step starts a new stage of the update and logs it.
func (r *updateReporter) Step(step string, progress int, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Println(message)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status.Step, r.status.Message = step, message
	if progress > r.status.Progress {
		r.status.Progress = progress
	}
	r.status.File, r.status.Downloaded, r.status.Total = "", 0, 0
	r.addLog("info", message)
	r.writeLocked(true)
}

// Info adds a log line without changing the stage.
func (r *updateReporter) Info(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Println(message)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.addLog("info", message)
	r.writeLocked(true)
}

// Download reports bytes received for one file; progress moves between from and to.
func (r *updateReporter) Download(file string, downloaded, total int64, from, to int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status.File, r.status.Downloaded, r.status.Total = file, downloaded, total
	if total > 0 && to > from {
		if p := from + int(int64(to-from)*downloaded/total); p > r.status.Progress {
			r.status.Progress = p
		}
	}
	r.writeLocked(false)
}

// Fail records the error that stopped the update.
func (r *updateReporter) Fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	r.status.State, r.status.Error, r.status.FinishedAt = UpdateStateFailed, err.Error(), &now
	r.status.Message = "Update failed during step " + r.status.Step
	r.addLog("error", err.Error())
	r.writeLocked(true)
}

// Done records a finished update and the version now installed.
func (r *updateReporter) Done(installed string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	r.status.State, r.status.Step, r.status.Progress, r.status.FinishedAt = UpdateStateDone, UpdateStepDone, 100, &now
	r.status.InstalledVersion = installed
	r.status.File, r.status.Downloaded, r.status.Total = "", 0, 0
	r.status.Message = "Update finished"
	if installed != "" {
		r.status.Message = "Updated to " + installed
	}
	r.addLog("info", r.status.Message)
	r.writeLocked(true)
}

// writeLocked replaces the status file atomically. Download progress is
// throttled; stage changes are always written.
func (r *updateReporter) writeLocked(force bool) {
	now := time.Now()
	if !force && now.Sub(r.lastWrite) < 400*time.Millisecond {
		return
	}
	r.lastWrite = now
	r.status.UpdatedAt = now.UTC()
	if err := writeUpdateStatus(r.dir, r.status); err != nil && !r.warned {
		r.warned = true
		fmt.Fprintf(os.Stderr, "update status is not reported to the panel: %v\n", err)
	}
}

func writeUpdateStatus(dir string, status UpdateStatus) error {
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".status-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, updateStatusName))
}

// readUpdateStatus returns the latest status, or nil when no update has
// reported one since the machine started.
func readUpdateStatus() (*UpdateStatus, error) {
	f, err := os.OpenFile(filepath.Join(updateRuntimeDir(), updateStatusName), os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("update status is not a regular file")
	}
	var status UpdateStatus
	if err := json.NewDecoder(io.LimitReader(f, maxUpdateStatusSize)).Decode(&status); err != nil {
		return nil, fmt.Errorf("read update status: %w", err)
	}
	return &status, nil
}

// progressWriter reports download progress as bytes arrive.
type progressWriter struct {
	w          io.Writer
	downloaded int64
	report     func(downloaded int64)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.downloaded += int64(n)
	p.report(p.downloaded)
	return n, err
}
