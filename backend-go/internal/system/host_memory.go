package system

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// hostMemory reports total, free and used bytes of the host kernel.
// Reading /proc/meminfo avoids runtime.ReadMemStats, which stops the world.
func hostMemory() (total, free, used uint64) {
	if total, free, available, ok := readProcMeminfo("/proc/meminfo"); ok {
		if available == 0 || available > total {
			// Kernels without MemAvailable: keep the upstream MemFree formula.
			available = free
		}
		return total, available, total - available
	}
	// Non-Linux development hosts: fall back to the process' own memory.
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Sys, m.Sys - m.Alloc, m.Alloc
}

func readProcMeminfo(path string) (total, free, available uint64, ok bool) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, 0, false
	}
	defer file.Close()

	var haveTotal, haveFree, haveAvailable bool
	scanner := bufio.NewScanner(file)
	for scanner.Scan() && !(haveTotal && haveFree && haveAvailable) {
		key, rest, found := strings.Cut(scanner.Text(), ":")
		if !found {
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
		switch key {
		case "MemTotal":
			total, haveTotal = kib*1024, true
		case "MemFree":
			free, haveFree = kib*1024, true
		case "MemAvailable":
			available, haveAvailable = kib*1024, true
		}
	}
	if scanner.Err() != nil || !haveTotal || !haveFree || free > total {
		return 0, 0, 0, false
	}
	return total, free, available, true
}
