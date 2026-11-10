package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gowave/internal/httpx"
	"gowave/internal/logx"
)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
}

// GitHub allows 60 anonymous API calls an hour per address, and every open
// dashboard asks for updates, so answers are kept for a while. Repeated checks
// send the previous ETag: a "304 Not Modified" answer does not count against
// the limit. A failed check is remembered briefly so open dashboards do not
// keep hitting a rate-limited API.
const (
	releaseCacheTTL      = 10 * time.Minute
	releaseErrorTTL      = time.Minute
	releaseForceInterval = 10 * time.Second
)

type cachedRelease struct {
	release   GitHubRelease
	etag      string
	fetched   time.Time
	err       error
	errAt     time.Time
	hasResult bool
}

var releaseCache = struct {
	sync.Mutex
	byChannel map[string]cachedRelease
}{byChannel: map[string]cachedRelease{}}

// releaseHTTPClient is replaced in tests.
var releaseHTTPClient = &http.Client{Timeout: 15 * time.Second}

// fetchRelease returns the newest release of a channel and when GitHub was last
// asked. A dev release always carries the "dev" tag, so its name holds the
// build version (dev-<sha>). force skips the cache unless it is only seconds old.
func fetchRelease(channel string, force bool) (GitHubRelease, time.Time, error) {
	releaseCache.Lock()
	defer releaseCache.Unlock()
	c := releaseCache.byChannel[channel]
	now := time.Now()
	if force {
		if c.hasResult && now.Sub(c.fetched) < releaseForceInterval {
			return c.release, c.fetched, nil
		}
		if c.err != nil && now.Sub(c.errAt) < releaseForceInterval {
			return GitHubRelease{}, c.errAt, c.err
		}
	} else {
		if c.hasResult && now.Sub(c.fetched) < releaseCacheTTL {
			return c.release, c.fetched, nil
		}
		if c.err != nil && now.Sub(c.errAt) < releaseErrorTTL {
			return GitHubRelease{}, c.errAt, c.err
		}
	}

	release, etag, notModified, err := requestRelease(channel, c.etag)
	if err != nil {
		c.err, c.errAt = err, now
		releaseCache.byChannel[channel] = c
		return GitHubRelease{}, now, err
	}
	if notModified && c.hasResult {
		release, etag = c.release, c.etag
	}
	releaseCache.byChannel[channel] = cachedRelease{release: release, etag: etag, fetched: now, hasResult: true}
	return release, now, nil
}

func requestRelease(channel, etag string) (release GitHubRelease, newETag string, notModified bool, err error) {
	req, err := http.NewRequest(http.MethodGet, releaseAPIURL(channel), nil)
	if err != nil {
		return GitHubRelease{}, "", false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "GoWave/"+AppVersion)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := releaseHTTPClient.Do(req)
	if err != nil {
		return GitHubRelease{}, "", false, fmt.Errorf("GitHub is not reachable from the server: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotModified && etag != "":
		return GitHubRelease{}, etag, true, nil
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusNotFound && channel == ChannelDev:
		return GitHubRelease{}, "", false, errors.New("GitHub has no dev build right now; a new one may be being published, try again in a few minutes")
	case (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests) && resp.Header.Get("X-RateLimit-Remaining") == "0":
		msg := "GitHub API rate limit for this server's address is exhausted (60 requests an hour)"
		if reset, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			msg += "; it resets at " + time.Unix(reset, 0).UTC().Format("15:04 UTC")
		}
		return GitHubRelease{}, "", false, errors.New(msg)
	default:
		return GitHubRelease{}, "", false, fmt.Errorf("GitHub releases returned %s", resp.Status)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&release); err != nil {
		return GitHubRelease{}, "", false, fmt.Errorf("read GitHub release: %w", err)
	}
	return release, resp.Header.Get("ETag"), false, nil
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

// noStore keeps browsers and proxies from caching update answers.
func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
}

// CheckUpdate reports the newest build of a channel: the current one, or the
// one named by ?channel= when the admin is about to switch between stable and
// dev. ?force=1 asks GitHub again instead of answering from the cache.
func (h *Handler) CheckUpdate(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	channel, err := parseChannel(r.URL.Query().Get("channel"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeInternal, err.Error())
		return
	}
	isDocker := IsRunningInDocker()
	if isDocker {
		channel = CurrentChannel()
	}
	force := r.URL.Query().Get("force") == "1" || r.URL.Query().Get("force") == "true"
	release, checkedAt, err := fetchRelease(channel, force)
	if err != nil {
		httpx.Error(w, r, http.StatusBadGateway, httpx.CodeInternal, "Could not check for updates: "+err.Error())
		return
	}
	latest := releaseVersion(channel, release)
	switching := channel != CurrentChannel()

	result := map[string]interface{}{
		"currentVersion": AppVersion,
		"latestVersion":  latest,
		"changelog":      release.Body,
		"channel":        CurrentChannel(),
		"targetChannel":  channel,
		"hasUpdate":      latest != "" && AppVersion != "unknown" && (switching || latest != AppVersion),
		"isDocker":       isDocker,
		"canAutoUpdate":  !isDocker && updateHelperInstalled(),
		"checkedAt":      checkedAt.UTC(),
	}
	if isDocker {
		result["updateCommand"] = DockerUpdateCommand()
	} else if !updateHelperInstalled() {
		result["updateCommand"] = installerCommand(channel)
	}
	httpx.JSON(w, http.StatusOK, result)
}

// DockerUpdateCommand is shown instead of in-panel updates: a container cannot
// replace its own image, and giving the panel the Docker socket would make a
// panel compromise a host compromise. GOWAVE_COMPOSE_DIR is written to .env by
// `gowave migrate to-docker`. The image tag in docker-compose.yml selects the
// channel and is changed manually on the host.
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

// UpdateHelperInstalled reports whether the root update helper is set up.
func UpdateHelperInstalled() bool { return updateHelperInstalled() }

func updateHelperInstalled() bool {
	_, err := os.Stat(updateHelperUnit)
	return err == nil
}

func (h *Handler) ApplyUpdate(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	// An optional {"channel": "stable"|"dev"} switches channels; without it the
	// panel updates within its own channel.
	var body struct {
		Channel string `json:"channel"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<10)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			httpx.Error(w, r, http.StatusBadRequest, httpx.CodeInternal, "Invalid request body")
			return
		}
	}
	channel, err := parseChannel(body.Channel)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeInternal, err.Error())
		return
	}
	if IsRunningInDocker() {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeInternal, "In-panel updates are not available in Docker. Update on the server: "+DockerUpdateCommand())
		return
	}
	if !updateHelperInstalled() {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeInternal, "The update helper is not installed. Update on the server: "+installerCommand(channel))
		return
	}

	// The request names only the channel; the root helper accepts nothing else
	// and downloads that channel's newest verified build.
	requestedAt := time.Now().UTC()
	if err := os.WriteFile(updateRequestPath(), []byte(channel), 0600); err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, httpx.CodeInternal, "Failed to request the update: "+err.Error())
		return
	}
	logx.Infof("update", "Panel update to the latest %s build requested by %s; progress: journalctl -u gowave-update.service", channel, httpx.ClientIP(r))

	httpx.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "channel": channel, "requestedAt": requestedAt})
}

// updateRuntimeDir holds the update request and the helper's status file.
func updateRuntimeDir() string {
	if runtimeDir := os.Getenv("RUNTIME_DIRECTORY"); runtimeDir != "" {
		dir, _, _ := strings.Cut(runtimeDir, ":")
		return dir
	}
	return updateRequestDir
}

func updateRequestPath() string {
	return filepath.Join(updateRuntimeDir(), updateRequestName)
}

// UpdateStatus reports the progress of an in-panel update: whether the request
// still waits for the root helper, and what the helper last reported.
func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	_, err := os.Lstat(updateRequestPath())
	result := map[string]interface{}{
		"currentVersion":  AppVersion,
		"channel":         CurrentChannel(),
		"helperInstalled": updateHelperInstalled(),
		"requestPending":  err == nil,
		"serverTime":      time.Now().UTC(),
		"status":          nil,
	}
	status, err := readUpdateStatus()
	if err != nil {
		result["statusError"] = err.Error()
	} else if status != nil {
		result["status"] = status
	}
	httpx.JSON(w, http.StatusOK, result)
}
