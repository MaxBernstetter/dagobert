package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// hostDataDirForJobs returns the host path to bind-mount into pipeline job containers.
// Compose may pass a relative HOST_DATA_DIR; job docker runs on the host via the socket.
func hostDataDirForJobs() (string, error) {
	raw := strings.TrimSpace(os.Getenv("HOST_DATA_DIR"))
	if raw == "" {
		return "", fmt.Errorf("HOST_DATA_DIR is required")
	}
	if filepath.IsAbs(raw) {
		return raw, nil
	}
	live := env("LIVE_DIR", "/data")
	if src, err := bindMountSource(live); err == nil && src != "" {
		return src, nil
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func bindMountSource(mountPoint string) (string, error) {
	mountPoint = filepath.Clean(mountPoint)
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		// mountinfo: … - <fstype> <mount point> <mount source> …
		fields := strings.Fields(sc.Text())
		if len(fields) < 10 {
			continue
		}
		mp := fields[4]
		if mp != mountPoint {
			continue
		}
		// Docker bind mounts record the host directory in the "root" field (see proc(5) mountinfo).
		root := fields[3]
		if plausibleHostDataDir(root) {
			return filepath.Clean(root), nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no bind mount for %s", mountPoint)
}

func plausibleHostDataDir(p string) bool {
	if p == "" || p == "/" || !filepath.IsAbs(p) {
		return false
	}
	if strings.HasPrefix(p, "/dev/") {
		return false
	}
	// Reject mistaken parse of mount options (old bug returned "rw").
	if strings.Contains(p, ",") {
		return false
	}
	return true
}
