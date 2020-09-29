package git

import (
	"os/exec"
	"strings"

	giturls "github.com/whilp/git-urls"
)

type GitRemote string

func (gr GitRemote) String() string {
	return string(gr)
}

func ProvideGitRemote() GitRemote {
	return GitRemote(normalizeGitRemote(gitOrigin(".")))
}

func gitOrigin(fromDir string) string {
	cmd := exec.Command("git", "-C", fromDir, "remote", "get-url", "origin")
	b, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimRight(string(b), "\n")
}

func normalizeGitRemote(s string) string {
	u, err := giturls.Parse(string(s))
	if err != nil {
		return s
	}

	// treat "http://", "https://", "git://", "ssh://", etc as equiv
	u.Scheme = ""

	u.User = nil

	// github.com/tilt-dev/tilt is the same as github.com/tilt-dev/tilt/
	if strings.HasSuffix(u.Path, "/") {
		u.Path = u.Path[:len(u.Path)-1]
	}

	// github.com/tilt-dev/tilt is the same as github.com/tilt-dev/tilt.git
	if strings.HasSuffix(u.Path, ".git") {
		u.Path = u.Path[:len(u.Path)-4]
	}

	return u.String()
}
