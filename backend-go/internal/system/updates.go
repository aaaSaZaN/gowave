package system

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
		"canAutoUpdate":  !isDocker && updateHelperInstalled(),
	}
	if isDocker {
		result["updateCommand"] = DockerUpdateCommand()
	} else if !updateHelperInstalled() {
		result["updateCommand"] = InstallerUpdateCommand
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

// InstallerUpdateCommand is shown when the native service has no update helper,
// for example on installations made before the helper existed.
const InstallerUpdateCommand = `sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/aaaSaZaN/gowave/main/install.sh)"`

// The panel runs unprivileged and cannot replace its root-owned binary. It only
// leaves a request in its runtime directory; gowave-update.path (installed by
// `gowave service install`) then starts a root oneshot that runs `update apply`.
var (
	updateHelperUnit = "/etc/systemd/system/gowave-update.path"
	updateRequestDir = "/run/gowave"
)

const updateRequestName = "update-request"

func updateHelperInstalled() bool {
	_, err := os.Stat(updateHelperUnit)
	return err == nil
}

func (h *Handler) ApplyUpdate(w http.ResponseWriter, r *http.Request) {
	if IsRunningInDocker() {
		http.Error(w, "In-panel updates are not available in Docker. Update on the server: "+DockerUpdateCommand(), http.StatusBadRequest)
		return
	}
	if !updateHelperInstalled() {
		http.Error(w, "The update helper is not installed. Update on the server: "+InstallerUpdateCommand, http.StatusConflict)
		return
	}

	dir := updateRequestDir
	if runtimeDir := os.Getenv("RUNTIME_DIRECTORY"); runtimeDir != "" {
		dir, _, _ = strings.Cut(runtimeDir, ":")
	}
	// The request carries no data: the helper always installs the latest release.
	if err := os.WriteFile(filepath.Join(dir, updateRequestName), nil, 0600); err != nil {
		http.Error(w, "Failed to request the update: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
