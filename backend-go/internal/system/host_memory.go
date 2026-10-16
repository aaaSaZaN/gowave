package system

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// hostMemory reports total, free and used bytes like upstream's
// systeminformation.mem(): free is MemFree and used is total minus free.
// Reading /proc/meminfo avoids runtime.ReadMemStats, which stops the world.
func hostMemory() (total, free, used uint64) {
	if total, free, ok := readProcMeminfo("/proc/meminfo"); ok {
		return total, free, total - free
	}
	// Non-Linux development hosts: fall back to the process' own memory.
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Sys, m.Sys - m.Alloc, m.Alloc
}

func readProcMeminfo(path string) (total, free uint64, ok bool) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, false
	}
	defer file.Close()

	var haveTotal, haveFree bool
	scanner := bufio.NewScanner(file)
	for scanner.Scan() && !(haveTotal && haveFree) {
		key, rest, found := strings.Cut(scanner.Text(), ":")
		if !found || (key != "MemTotal" && key != "MemFree") {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		kib, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		if key == "MemTotal" {
			total, haveTotal = kib*1024, true
		} else {
			free, haveFree = kib*1024, true
		}
	}
	if !haveTotal || !haveFree || free > total {
		return 0, 0, false
	}
	return total, free, true
}
