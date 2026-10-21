package nodes

import (
	"regexp"
	"strings"
)

// xrayConfigInvalidPrefix starts the error Remnanode (Go) returns when
// "xray run -test" rejects a config.
const xrayConfigInvalidPrefix = "invalid Xray configuration"

// maxStatusMessageLength caps what is stored for display; the full error is
// logged by the panel and the node.
const maxStatusMessageLength = 1000

var xrayLoadFilesPrefix = regexp.MustCompile(`^main: failed to load config files: \[[^\]]*\] > `)

// ConfigRejectedError reports a start the node refused while its running Xray
// kept serving the previous config.
type ConfigRejectedError struct {
	Reason string
	// Invalid is set when Xray rejected the config itself, so retrying the
	// same config cannot succeed.
	Invalid bool
}

func (e *ConfigRejectedError) Error() string {
	return "node kept the previous Xray config: " + e.Reason
}

// summarizeXrayError reduces a start error to the line Xray prints for a
// config it cannot load ("Failed to start: ..."), dropping the version banner
// and log lines of "xray run -test". Other errors are kept as they are.
func summarizeXrayError(message string) string {
	summary := strings.TrimSpace(message)
	for _, line := range strings.Split(message, "\n") {
		if i := strings.Index(line, "Failed to start: "); i >= 0 {
			summary = xrayLoadFilesPrefix.ReplaceAllString(strings.TrimSpace(line[i+len("Failed to start: "):]), "")
			break
		}
	}
	if runes := []rune(summary); len(runes) > maxStatusMessageLength {
		summary = string(runes[:maxStatusMessageLength]) + "…"
	}
	return summary
}
