package system

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
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
// root-owned, reapplies the service policy and restarts the panel.
func ApplyRelease(installDir string) error {
	if os.Geteuid() != 0 {
		return errors.New("update apply must run as root")
	}
	installDir = filepath.Clean(installDir)
	exe, err := trustedExecutable(installDir)
	if err != nil {
		return err
	}
	// The request only names the channel; it has served its purpose once read.
	request := filepath.Join(updateRequestDir, updateRequestName)
	channel := readRequestedChannel(request)
	_ = os.Remove(request)
	fmt.Printf("Installing the latest %s build\n", channel)
	baseURL := releaseDownloadBase(channel)

	work, err := os.MkdirTemp("", "gowave-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	binName := "gowave-linux-" + runtime.GOARCH
	const frontName = "gowave-frontend.tar.gz"
	client := &http.Client{Timeout: 10 * time.Minute}
	for _, name := range []string{"SHA256SUMS", binName, frontName} {
		if err := download(client, baseURL+name, filepath.Join(work, name)); err != nil {
			return fmt.Errorf("download %s: %w", name, err)
		}
	}
	sums, err := readChecksums(filepath.Join(work, "SHA256SUMS"))
	if err != nil {
		return err
	}
	for _, name := range []string{binName, frontName} {
		if err := verifyChecksum(filepath.Join(work, name), sums[name]); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}

	if err := installFrontend(filepath.Join(work, frontName), filepath.Join(installDir, "frontend")); err != nil {
		return fmt.Errorf("install frontend: %w", err)
	}
	if err := installBinary(filepath.Join(work, binName), exe); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}

	for _, args := range [][]string{
		{exe, "service", "install", installDir},
		{"systemctl", "restart", "gowave.service"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %w", strings.Join(args, " "), err)
		}
	}
	return nil
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

func download(client *http.Client, url, dest string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	n, err := io.Copy(out, io.LimitReader(resp.Body, maxReleaseFile+1))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil && n > maxReleaseFile {
		err = errors.New("file is too large")
	}
	return err
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
