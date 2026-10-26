package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
}

// GitHub allows 60 anonymous API calls an hour per address, and every open
// dashboard asks for updates, so answers are kept for a while.
const releaseCacheTTL = 10 * time.Minute

type cachedRelease struct {
	release GitHubRelease
	fetched time.Time
}

var releaseCache = struct {
	sync.Mutex
	byChannel map[string]cachedRelease
}{byChannel: map[string]cachedRelease{}}

// fetchRelease returns the newest release of a channel. A dev release always
// carries the "dev" tag, so its name holds the build version (dev-<sha>).
func fetchRelease(channel string) (GitHubRelease, error) {
	releaseCache.Lock()
	defer releaseCache.Unlock()
	if c, ok := releaseCache.byChannel[channel]; ok && time.Since(c.fetched) < releaseCacheTTL {
		return c.release, nil
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(releaseAPIURL(channel))
	if err != nil {
		return GitHubRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return GitHubRelease{}, fmt.Errorf("GitHub releases returned %s", resp.Status)
	}
	var release GitHubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&release); err != nil {
		return GitHubRelease{}, err
	}
	releaseCache.byChannel[channel] = cachedRelease{release: release, fetched: time.Now()}
	return release, nil
}

func releaseVersion(channel string, release GitHubRelease) string {
	if channel == ChannelDev {
		return release.Name
	}
	return release.TagName
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

// CheckUpdate reports the newest build of a channel: the current one, or the
// one named by ?channel= when the admin is about to switch between stable and dev.
func (h *Handler) CheckUpdate(w http.ResponseWriter, r *http.Request) {
	channel, err := parseChannel(r.URL.Query().Get("channel"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	release, err := fetchRelease(channel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	latest := releaseVersion(channel, release)
	switching := channel != CurrentChannel()

	isDocker := IsRunningInDocker()
	result := map[string]interface{}{
		"currentVersion": AppVersion,
		"latestVersion":  latest,
		"changelog":      release.Body,
		"channel":        CurrentChannel(),
		"targetChannel":  channel,
		"hasUpdate":      latest != "" && AppVersion != "unknown" && (switching || latest != AppVersion),
		"isDocker":       isDocker,
		"canAutoUpdate":  !isDocker && updateHelperInstalled(),
	}
	if isDocker {
		result["updateCommand"] = DockerUpdateCommand(channel)
	} else if !updateHelperInstalled() {
		result["updateCommand"] = installerCommand(channel)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// DockerUpdateCommand is shown instead of in-panel updates: a container cannot
// replace its own image, and giving the panel the Docker socket would make a
// panel compromise a host compromise. GOWAVE_COMPOSE_DIR is written to .env by
// `gowave migrate to-docker`. Switching channels rewrites GOWAVE_IMAGE there.
func DockerUpdateCommand(channel string) string {
	dir := strings.TrimSpace(os.Getenv("GOWAVE_COMPOSE_DIR"))
	if dir == "" {
		dir = "/path/to/gowave-compose-dir"
	}
	if channel == CurrentChannel() {
		return fmt.Sprintf("cd %s && docker compose pull && docker compose up -d", dir)
	}
	return fmt.Sprintf("cd %s && sed -i '/^GOWAVE_IMAGE=/d' .env && echo 'GOWAVE_IMAGE=%s:%s' >> .env && docker compose pull && docker compose up -d",
		dir, dockerImage, dockerTag(channel))
}

// InstallerUpdateCommand is shown when the native service has no update helper,
// for example on installations made before the helper existed.
const InstallerUpdateCommand = `sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/aaaSaZaN/gowave/main/install.sh)"`

// installerCommand updates without prompts and lands on the given channel.
func installerCommand(channel string) string {
	return InstallerUpdateCommand + " _ --update --" + channel
}

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
	// An optional {"channel": "stable"|"dev"} switches channels; without it the
	// panel updates within its own channel.
	var body struct {
		Channel string `json:"channel"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<10)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
	}
	channel, err := parseChannel(body.Channel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if IsRunningInDocker() {
		http.Error(w, "In-panel updates are not available in Docker. Update on the server: "+DockerUpdateCommand(channel), http.StatusBadRequest)
		return
	}
	if !updateHelperInstalled() {
		http.Error(w, "The update helper is not installed. Update on the server: "+installerCommand(channel), http.StatusConflict)
		return
	}

	dir := updateRequestDir
	if runtimeDir := os.Getenv("RUNTIME_DIRECTORY"); runtimeDir != "" {
		dir, _, _ = strings.Cut(runtimeDir, ":")
	}
	// The request names only the channel; the root helper accepts nothing else
	// and downloads that channel's newest verified build.
	if err := os.WriteFile(filepath.Join(dir, updateRequestName), []byte(channel), 0600); err != nil {
		http.Error(w, "Failed to request the update: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
