// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package get

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tilt-dev/go-get/internal/web"
)

func init() {
	// Disable any prompting for passwords by Git.
	// Only has an effect for 2.3.0 or later, but avoiding
	// the prompt in earlier versions is just too hard.
	// If user has explicitly set GIT_TERMINAL_PROMPT=1, keep
	// prompting.
	// See golang.org/issue/9341 and golang.org/issue/12706.
	if os.Getenv("GIT_TERMINAL_PROMPT") == "" {
		os.Setenv("GIT_TERMINAL_PROMPT", "0")
	}

	// Disable any ssh connection pooling by Git.
	// If a Git subprocess forks a child into the background to cache a new connection,
	// that child keeps stdout/stderr open. After the Git subprocess exits,
	// os /exec expects to be able to read from the stdout/stderr pipe
	// until EOF to get all the data that the Git subprocess wrote before exiting.
	// The EOF doesn't come until the child exits too, because the child
	// is holding the write end of the pipe.
	// This is unfortunate, but it has come up at least twice
	// (see golang.org/issue/13453 and golang.org/issue/16104)
	// and confuses users when it does.
	// If the user has explicitly set GIT_SSH or GIT_SSH_COMMAND,
	// assume they know what they are doing and don't step on it.
	// But default to turning off ControlMaster.
	if os.Getenv("GIT_SSH") == "" && os.Getenv("GIT_SSH_COMMAND") == "" {
		os.Setenv("GIT_SSH_COMMAND", "ssh -o ControlMaster=no")
	}
}

// Downloader fetches repositories under the given source tree.
// Not thread-safe.
type Downloader struct {
	srcRoot string

	// downloadRootCache records the version control repository
	// root directories we have already considered during the download.
	// For example, all the packages in the github.com/google/codesearch repo
	// share the same root (the directory for that path), and we only need
	// to run the hg commands to consider each repository once.
	downloadRootCache map[string]bool
}

func NewDownloader(srcRoot string) Downloader {
	return Downloader{
		srcRoot:           srcRoot,
		downloadRootCache: map[string]bool{},
	}
}

// downloadPackage runs the create or download command
// to make the first copy of or update a copy of the given package.
func (d *Downloader) Download(pkg string) (string, error) {
	var (
		vcs            *vcsCmd
		repo, rootPath string
		err            error
	)

	security := web.SecureOnly
	srcRoot := d.srcRoot
	if i := strings.Index(pkg, "..."); i >= 0 {
		slash := strings.LastIndexByte(pkg[:i], '/')
		if slash < 0 {
			return "", fmt.Errorf("cannot expand ... in %q", pkg)
		}
		pkg = pkg[:slash]
	}
	if err := checkImportPath(pkg); err != nil {
		return "", fmt.Errorf("%s: invalid import path: %v", pkg, err)
	}

	// Analyze the import path to determine the version control system,
	// repository, and the import path for the root of the repository.
	rr, err := repoRootForImportPath(pkg, security)
	if err != nil {
		return "", err
	}
	vcs, repo, rootPath = rr.vcs, rr.Repo, rr.Root

	result := filepath.Join(srcRoot, filepath.FromSlash(pkg))
	root := filepath.Join(srcRoot, filepath.FromSlash(rootPath))

	if err := checkNestedVCS(vcs, root, srcRoot); err != nil {
		return "", err
	}

	// If we've considered this repository already, don't do it again.
	if d.downloadRootCache[root] {
		return result, nil
	}
	d.downloadRootCache[root] = true

	// Check that this is an appropriate place for the repo to be checked out.
	// The target directory must either not exist or have a repo checked out already.
	meta := filepath.Join(root, "."+vcs.cmd)
	if _, err := os.Stat(meta); err != nil {
		// Metadata file or directory does not exist. Prepare to checkout new copy.
		// Some version control tools require the target directory not to exist.
		// We require that too, just to avoid stepping on existing work.
		if _, err := os.Stat(root); err == nil {
			return "", fmt.Errorf("%s exists but %s does not - stale checkout?", root, meta)
		}

		// Some version control tools require the parent of the target to exist.
		parent, _ := filepath.Split(root)
		if err = os.MkdirAll(parent, 0777); err != nil {
			return "", err
		}

		if err = vcs.create(root, repo); err != nil {
			return "", err
		}
	} else {
		// Metadata directory does exist; download incremental updates.
		if err = vcs.download(root); err != nil {
			return "", err
		}
	}

	// Select and sync to appropriate version of the repository.
	if err := vcs.tagSync(root, ""); err != nil {
		return "", err
	}

	return result, nil
}
