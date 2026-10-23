package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/MaxBernstetter/trading-copilot/src/shared"
)

func dockerJobUserArgs() []string {
	uid := strings.TrimSpace(os.Getenv("HOST_UID"))
	if uid == "" || uid == "0" {
		return nil
	}
	gid := strings.TrimSpace(os.Getenv("HOST_GID"))
	if gid == "" {
		gid = uid
	}
	return []string{"--user", uid + ":" + gid}
}

// ensureJobDataLayout prepares HOST_DATA_DIR on the host before job containers write
// under /data. Uses the host UID/GID when set so bind mounts stay user-owned.
func ensureJobDataLayout(hostDataDir string) error {
	if hostDataDir == "" {
		return fmt.Errorf("host data dir is required")
	}
	uid := strings.TrimSpace(os.Getenv("HOST_UID"))
	gid := strings.TrimSpace(os.Getenv("HOST_GID"))
	if uid != "" && uid != "0" {
		if gid == "" {
			gid = uid
		}
		cmd := exec.Command(
			"docker", "run", "--rm",
			"-v", hostDataDir+":/data",
			"--user", uid+":"+gid,
			"alpine:3.20",
			"mkdir", "-p", "/data/"+shared.DataParquetSubdir,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return shared.EnsureDataLayout(env("LIVE_DIR", "/data"))
}
