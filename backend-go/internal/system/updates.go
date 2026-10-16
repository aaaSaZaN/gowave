package system

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
}

// IsRunningInDocker checks whether the process is running inside a Docker or container environment
func IsRunningInDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		if strings.Contains(content, "docker") || strings.Contains(content, "containerd") || strings.Contains(content, "kubepods") {
			return true
		}
	}
	if os.Getenv("DOCKER") != "" || os.Getenv("IN_DOCKER") != "" || os.Getenv("CONTAINER") != "" {
		return true
	}
	return false
}

func (h *Handler) CheckUpdate(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get("https://api.github.com/repos/aaaSaZaN/gowave/releases/latest")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "GitHub releases returned "+resp.Status, http.StatusBadGateway)
		return
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	isDocker := IsRunningInDocker()
	result := map[string]interface{}{
		"currentVersion": AppVersion,
		"latestVersion":  release.TagName,
		"changelog":      release.Body,
		"hasUpdate":      release.TagName != "" && release.TagName != AppVersion && AppVersion != "unknown",
		"isDocker":       isDocker,
		"canAutoUpdate":  !isDocker,
	}
	if isDocker {
		result["updateCommand"] = DockerUpdateCommand()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// DockerUpdateCommand is shown instead of in-panel updates: a container cannot
// replace its own image, and giving the panel the Docker socket would make a
// panel compromise a host compromise. GOWAVE_COMPOSE_DIR is written to .env by
// `gowave migrate to-docker`.
func DockerUpdateCommand() string {
	dir := strings.TrimSpace(os.Getenv("GOWAVE_COMPOSE_DIR"))
	if dir == "" {
		dir = "/path/to/gowave-compose-dir"
	}
	return fmt.Sprintf("cd %s && docker compose pull && docker compose up -d", dir)
}

func (h *Handler) ApplyUpdate(w http.ResponseWriter, r *http.Request) {
	if IsRunningInDocker() {
		http.Error(w, "In-panel updates are not available in Docker. Update on the server: "+DockerUpdateCommand(), http.StatusBadRequest)
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	exePath, _ = filepath.EvalSymlinks(exePath)

	arch := runtime.GOARCH
	binURL := fmt.Sprintf("https://github.com/aaaSaZaN/gowave/releases/latest/download/gowave-linux-%s", arch)
	frontURL := "https://github.com/aaaSaZaN/gowave/releases/latest/download/gowave-frontend.tar.gz"

	tmpBin := exePath + ".new"
	if err := downloadFile(tmpBin, binURL); err != nil {
		http.Error(w, "Failed to download binary: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_ = os.Chmod(tmpBin, 0755)

	baseDir := filepath.Dir(exePath)
	targetDir := filepath.Join(baseDir, "frontend", "dist")
	_ = os.MkdirAll(targetDir, 0755)

	if err := downloadAndExtractTarGz(frontURL, targetDir); err != nil {
		_ = os.Remove(tmpBin)
		http.Error(w, "Failed to extract frontend: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Also sync into ./dist if it exists alongside, so all static configurations stay in sync
	altDist := filepath.Join(baseDir, "dist")
	if info, err := os.Stat(altDist); err == nil && info.IsDir() {
		_ = downloadAndExtractTarGz(frontURL, altDist)
	}

	_ = os.Rename(exePath, exePath+".bak")
	if err := os.Rename(tmpBin, exePath); err != nil {
		_ = os.Rename(exePath+".bak", exePath)
		http.Error(w, "Failed to replace binary: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})

	go func() {
		time.Sleep(2 * time.Second)
		_ = exec.Command("systemctl", "restart", "gowave").Run()
		os.Exit(0)
	}()
}

func downloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func downloadAndExtractTarGz(url, dest string) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("curl -sL %s | tar -xz -C %s", url, dest))
	return cmd.Run()
}
