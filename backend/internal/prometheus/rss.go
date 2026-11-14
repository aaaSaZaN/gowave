package prometheus

import (
	"os"
	"strconv"
	"strings"
)

// residentSetSize reads the RSS like process.memoryUsage().rss; outside
// Linux it falls back to the memory the Go runtime obtained.
func residentSetSize(fallback uint64) uint64 {
	data, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return fallback
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return fallback
	}
	pages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return fallback
	}
	return pages * uint64(os.Getpagesize())
}
