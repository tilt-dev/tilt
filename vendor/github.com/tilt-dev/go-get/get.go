// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package get

import (
	"fmt"
	"io"
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

	// ISSUE 1
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
	//
	// ISSUE 2
	// Do not attempt to prompt the user.
	// If a Git subprocess blocks for user interaction for a question (e.g.
	// accept host key, provide SSH keyfile passphrase), the download can hang
	// indefinitely, as the prompt.
	// Arguably, this should be configurable on the Downloader instance, but
	// it suits our needs to _always_ run in non-interactive mode, so it's
	// always applied, as this package isn't particularly amenable to making
	// it conditional.
	//
	// If the user has explicitly set GIT_SSH or GIT_SSH_COMMAND,
	// assume they know what they are doing and don't step on it.
	// But default to turning off ControlMaster and turning on BatchMode.
	if os.Getenv("GIT_SSH") == "" && os.Getenv("GIT_SSH_COMMAND") == "" {
		os.Setenv("GIT_SSH_COMMAND", "ssh -o ControlMaster=no -o BatchMode=yes")
	}
}

// Downloader fetches repositories under the given source tree.
// Not thread-safe.
type Downloader struct {
	Stderr io.Writer

	srcRoot string
}

func NewDownloader(srcRoot string) *Downloader {
	return &Downloader{
		Stderr:  os.Stderr,
		srcRoot: srcRoot,
	}
}

// Analyze the import path to determine the version control system,
// repository, and the import path for the root of the repository.
func (d *Downloader) repoRoot(pkg string) (string, *repoRoot, error) {
	security := web.SecureOnly
	if i := strings.Index(pkg, "..."); i >= 0 {
		slash := strings.LastIndexByte(pkg[:i], '/')
		if slash < 0 {
			return "", nil, fmt.Errorf("cannot expand ... in %q", pkg)
		}
		pkg = pkg[:slash]
	}
	if err := checkImportPath(pkg); err != nil {
		return "", nil, fmt.Errorf("%s: invalid import path: %v", pkg, err)
	}

	rr, err := repoRootForImportPath(pkg, security, d.Stderr)
	if err != nil {
		return "", nil, err
	}
	return pkg, rr, err
}

// Download runs the create or download command to make the first copy of or
// update a copy of the given package.
func (d *Downloader) Download(pkg string) (string, error) {
	var (
		vcs            *vcsCmd
		repo, rootPath string
		err            error
	)

	srcRoot := d.srcRoot
	pkg, rr, err := d.repoRoot(pkg)
	if err != nil {
		return "", err
	}
	vcs, repo, rootPath = rr.vcs, rr.Repo, rr.Root

	result := filepath.Join(srcRoot, filepath.FromSlash(pkg))
	root := filepath.Join(srcRoot, filepath.FromSlash(rootPath))

	if err := checkNestedVCS(vcs, root, srcRoot); err != nil {
		return "", err
	}

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

		if err = vcs.create(root, repo, d.Stderr); err != nil {
			return "", err
		}
	} else {
		// Metadata directory does exist; download incremental updates.
		if err = vcs.download(root, d.Stderr); err != nil {
			return "", err
		}
	}

	// Select and sync to appropriate version of the repository.
	if err := vcs.tagSync(root, "", d.Stderr); err != nil {
		return "", err
	}

	return result, nil
}

// Update the checked out repo to the given ref.
// Assumes the repo has already been downloaded.
func (d *Downloader) RefSync(pkg, tag string) error {
	srcRoot := d.srcRoot
	_, rr, err := d.repoRoot(pkg)
	if err != nil {
		return err
	}
	vcs, rootPath := rr.vcs, rr.Root
	root := filepath.Join(srcRoot, filepath.FromSlash(rootPath))
	cmdCtx := newCmdContext(root, d.Stderr)
	for _, cmd := range vcs.tagSyncCmd {
		if err := vcs.run(cmdCtx, cmd, "tag", tag); err != nil {
			return err
		}
	}
	return nil
}

// Determines where the repository will be downloaded before we download it.
func (d *Downloader) DestinationPath(pkg string) string {
	srcRoot := d.srcRoot
	return filepath.Join(srcRoot, filepath.FromSlash(pkg))
}

// Determines the hash of the currently checked out head.
//
// Returns the empty string if the current VCS does not support HEAD references.
func (d *Downloader) HeadRef(pkg string) (string, error) {
	srcRoot := d.srcRoot
	_, rr, err := d.repoRoot(pkg)
	if err != nil {
		return "", err
	}
	vcs := rr.vcs
	if vcs == nil || vcs.cmd != "git" {
		return "", nil
	}
	rootPath := rr.Root
	root := filepath.Join(srcRoot, filepath.FromSlash(rootPath))
	out, err := vcs.runOutput(d.toCmdContext(root), "rev-parse HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (d *Downloader) toCmdContext(dir string) cmdContext {
	return cmdContext{stderr: d.Stderr, dir: dir}
}
