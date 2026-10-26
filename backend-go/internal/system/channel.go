package system

import (
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
)

// Update channels. Stable builds come from v* tags and are what releases/latest
// and the Docker "latest" tag point to. Dev builds are rebuilt from every push
// to main into the rolling "dev" pre-release and the Docker "dev" tag.
const (
	ChannelStable = "stable"
	ChannelDev    = "dev"
)

// Set at build time by the dev workflow:
// -X gowave/internal/system.Channel=dev -X gowave/internal/system.CommitSha=<sha>
var (
	Channel   = ChannelStable
	CommitSha = ""
)

const (
	githubRepo  = "aaaSaZaN/gowave"
	dockerImage = "ghcr.io/aaasazan/gowave"
)

// CurrentChannel is the channel this binary was built for.
func CurrentChannel() string {
	if Channel == ChannelDev {
		return ChannelDev
	}
	return ChannelStable
}

func validChannel(channel string) bool {
	return channel == ChannelStable || channel == ChannelDev
}

func releaseDownloadBase(channel string) string {
	if channel == ChannelDev {
		return "https://github.com/" + githubRepo + "/releases/download/dev/"
	}
	return "https://github.com/" + githubRepo + "/releases/latest/download/"
}

func releaseAPIURL(channel string) string {
	if channel == ChannelDev {
		return "https://api.github.com/repos/" + githubRepo + "/releases/tags/dev"
	}
	return "https://api.github.com/repos/" + githubRepo + "/releases/latest"
}

func dockerTag(channel string) string {
	if channel == ChannelDev {
		return ChannelDev
	}
	return "latest"
}

// readRequestedChannel reads the channel the panel asked for. The request file
// lives in a directory the service account controls, so it is opened without
// following links, must be a small regular file, and anything but a known
// channel name falls back to the channel of the installed build.
func readRequestedChannel(name string) string {
	f, err := os.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return CurrentChannel()
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		return CurrentChannel()
	}
	data, err := io.ReadAll(io.LimitReader(f, 16))
	if err != nil {
		return CurrentChannel()
	}
	if channel := strings.TrimSpace(string(data)); validChannel(channel) {
		return channel
	}
	return CurrentChannel()
}

func parseChannel(channel string) (string, error) {
	channel = strings.TrimSpace(channel)
	if channel == "" {
		return CurrentChannel(), nil
	}
	if !validChannel(channel) {
		return "", fmt.Errorf("unknown update channel %q: use %q or %q", channel, ChannelStable, ChannelDev)
	}
	return channel, nil
}
