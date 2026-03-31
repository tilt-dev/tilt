//go:build !windows

package confutil

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// sudoer returns the user that invoked the current process with sudo only if
// sudo HOME env matches the home directory of the user that ran sudo and is
// part of configDir.
func sudoer(configDir string) *chowner {
	if _, ok := os.LookupEnv("SUDO_COMMAND"); !ok {
		return nil
	}
	suidenv := os.Getenv("SUDO_UID") // https://www.sudo.ws/docs/man/sudo.man/#SUDO_UID
	sgidenv := os.Getenv("SUDO_GID") // https://www.sudo.ws/docs/man/sudo.man/#SUDO_GID
	if suidenv == "" || sgidenv == "" {
		return nil
	}
	u, err := user.LookupId(suidenv)
	if err != nil {
		return nil
	}
	suid, err := strconv.Atoi(suidenv)
	if err != nil {
		return nil
	}
	sgid, err := strconv.Atoi(sgidenv)
	if err != nil {
		return nil
	}
	home, _ := os.UserHomeDir()
	if home == "" || u.HomeDir != home {
		return nil
	}
	if ok, _ := isSubPath(home, configDir); !ok {
		return nil
	}
	return &chowner{uid: suid, gid: sgid}
}

func fileOwner(fi os.FileInfo) *chowner {
	st := fi.Sys().(*syscall.Stat_t)
	return &chowner{uid: int(st.Uid), gid: int(st.Gid)}
}

func isSubPath(basePath, subPath string) (bool, error) {
	rel, err := filepath.Rel(basePath, subPath)
	if err != nil {
		return false, err
	}
	return !strings.HasPrefix(rel, "..") && rel != ".", nil
}
