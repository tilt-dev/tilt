// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package get

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	urlpkg "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tilt-dev/go-get/internal/web"
)

type cmdContext struct {
	dir    string
	stderr io.Writer
}

func newCmdContext(dir string, stderr io.Writer) cmdContext {
	return cmdContext{stderr: stderr, dir: dir}
}

// A vcsCmd describes how to use a version control system
// like Mercurial, Git, or Subversion.
type vcsCmd struct {
	name string
	cmd  string // name of binary to invoke command

	createCmd   []string // commands to download a fresh copy of a repository
	downloadCmd []string // commands to download updates into an existing repository

	tagCmd         []tagCmd // commands to list tags
	tagLookupCmd   []tagCmd // commands to lookup tags before running tagSyncCmd
	tagSyncCmd     []string // commands to sync to specific tag
	tagSyncDefault []string // commands to sync to default tag

	scheme  []string
	pingCmd string

	remoteRepo  func(v *vcsCmd, rootDir cmdContext) (remoteRepo string, err error)
	resolveRepo func(v *vcsCmd, rootDir cmdContext, remoteRepo string) (realRepo string, err error)
}

var defaultSecureScheme = map[string]bool{
	"https":   true,
	"git+ssh": true,
	"bzr+ssh": true,
	"svn+ssh": true,
	"ssh":     true,
}

func (v *vcsCmd) isSecure(repo string) bool {
	u, err := urlpkg.Parse(repo)
	if err != nil {
		// If repo is not a URL, it's not secure.
		return false
	}
	return v.isSecureScheme(u.Scheme)
}

func (v *vcsCmd) isSecureScheme(scheme string) bool {
	switch v.cmd {
	case "git":
		// GIT_ALLOW_PROTOCOL is an environment variable defined by Git. It is a
		// colon-separated list of schemes that are allowed to be used with git
		// fetch/clone. Any scheme not mentioned will be considered insecure.
		if allow := os.Getenv("GIT_ALLOW_PROTOCOL"); allow != "" {
			for _, s := range strings.Split(allow, ":") {
				if s == scheme {
					return true
				}
			}
			return false
		}
	}
	return defaultSecureScheme[scheme]
}

// A tagCmd describes a command to list available tags
// that can be passed to tagSyncCmd.
type tagCmd struct {
	cmd     string // command to list tags
	pattern string // regexp to extract tags from list
}

// vcsList lists the known version control systems
var vcsList = []*vcsCmd{
	vcsHg,
	vcsGit,
	vcsSvn,
	vcsBzr,
	vcsFossil,
}

// vcsByCmd returns the version control system for the given
// command name (hg, git, svn, bzr).
func vcsByCmd(cmd string) *vcsCmd {
	for _, vcs := range vcsList {
		if vcs.cmd == cmd {
			return vcs
		}
	}
	return nil
}

// vcsHg describes how to use Mercurial.
var vcsHg = &vcsCmd{
	name: "Mercurial",
	cmd:  "hg",

	createCmd:   []string{"clone -U -- {repo} {dir}"},
	downloadCmd: []string{"pull"},

	// We allow both tag and branch names as 'tags'
	// for selecting a version. This lets people have
	// a go.release.r60 branch and a go1 branch
	// and make changes in both, without constantly
	// editing .hgtags.
	tagCmd: []tagCmd{
		{"tags", `^(\S+)`},
		{"branches", `^(\S+)`},
	},
	tagSyncCmd:     []string{"update -r {tag}"},
	tagSyncDefault: []string{"update default"},

	scheme:     []string{"https", "http", "ssh"},
	pingCmd:    "identify -- {scheme}://{repo}",
	remoteRepo: hgRemoteRepo,
}

func hgRemoteRepo(vcsHg *vcsCmd, rootDir cmdContext) (remoteRepo string, err error) {
	out, err := vcsHg.runOutput(rootDir, "paths default")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// vcsGit describes how to use Git.
var vcsGit = &vcsCmd{
	name: "Git",
	cmd:  "git",

	createCmd:   []string{"clone -- {repo} {dir}", "-go-internal-cd {dir} submodule update --init --recursive"},
	downloadCmd: []string{"pull --ff-only", "submodule update --init --recursive"},

	tagCmd: []tagCmd{
		// tags/xxx matches a git tag named xxx
		// origin/xxx matches a git branch named xxx on the default remote repository
		{"show-ref", `(?:tags|origin)/(\S+)$`},
	},
	tagLookupCmd: []tagCmd{
		{"show-ref tags/{tag} origin/{tag}", `((?:tags|origin)/\S+)$`},
	},
	tagSyncCmd: []string{"checkout {tag}", "submodule update --init --recursive"},
	// both createCmd and downloadCmd update the working dir.
	// No need to do more here. We used to 'checkout master'
	// but that doesn't work if the default branch is not named master.
	// DO NOT add 'checkout master' here.
	// See golang.org/issue/9032.
	tagSyncDefault: []string{"submodule update --init --recursive"},

	scheme: []string{"git", "https", "http", "git+ssh", "ssh"},

	// Leave out the '--' separator in the ls-remote command: git 2.7.4 does not
	// support such a separator for that command, and this use should be safe
	// without it because the {scheme} value comes from the predefined list above.
	// See golang.org/issue/33836.
	pingCmd: "ls-remote {scheme}://{repo}",

	remoteRepo: gitRemoteRepo,
}

// scpSyntaxRe matches the SCP-like addresses used by Git to access
// repositories by SSH.
var scpSyntaxRe = regexp.MustCompile(`^([a-zA-Z0-9_]+)@([a-zA-Z0-9._-]+):(.*)$`)

func gitRemoteRepo(vcsGit *vcsCmd, rootDir cmdContext) (remoteRepo string, err error) {
	cmd := "config remote.origin.url"
	errParse := errors.New("unable to parse output of git " + cmd)
	errRemoteOriginNotFound := errors.New("remote origin not found")
	outb, err := vcsGit.run1(rootDir, cmd, nil, false)
	if err != nil {
		// if it doesn't output any message, it means the config argument is correct,
		// but the config value itself doesn't exist
		if outb != nil && len(outb) == 0 {
			return "", errRemoteOriginNotFound
		}
		return "", err
	}
	out := strings.TrimSpace(string(outb))

	var repoURL *urlpkg.URL
	if m := scpSyntaxRe.FindStringSubmatch(out); m != nil {
		// Match SCP-like syntax and convert it to a URL.
		// Eg, "git@github.com:user/repo" becomes
		// "ssh://git@github.com/user/repo".
		repoURL = &urlpkg.URL{
			Scheme: "ssh",
			User:   urlpkg.User(m[1]),
			Host:   m[2],
			Path:   m[3],
		}
	} else {
		repoURL, err = urlpkg.Parse(out)
		if err != nil {
			return "", err
		}
	}

	// Iterate over insecure schemes too, because this function simply
	// reports the state of the repo. If we can't see insecure schemes then
	// we can't report the actual repo URL.
	for _, s := range vcsGit.scheme {
		if repoURL.Scheme == s {
			return repoURL.String(), nil
		}
	}
	return "", errParse
}

// vcsBzr describes how to use Bazaar.
var vcsBzr = &vcsCmd{
	name: "Bazaar",
	cmd:  "bzr",

	createCmd: []string{"branch -- {repo} {dir}"},

	// Without --overwrite bzr will not pull tags that changed.
	// Replace by --overwrite-tags after http://pad.lv/681792 goes in.
	downloadCmd: []string{"pull --overwrite"},

	tagCmd:         []tagCmd{{"tags", `^(\S+)`}},
	tagSyncCmd:     []string{"update -r {tag}"},
	tagSyncDefault: []string{"update -r revno:-1"},

	scheme:      []string{"https", "http", "bzr", "bzr+ssh"},
	pingCmd:     "info -- {scheme}://{repo}",
	remoteRepo:  bzrRemoteRepo,
	resolveRepo: bzrResolveRepo,
}

func bzrRemoteRepo(vcsBzr *vcsCmd, rootDir cmdContext) (remoteRepo string, err error) {
	outb, err := vcsBzr.runOutput(rootDir, "config parent_location")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(outb)), nil
}

func bzrResolveRepo(vcsBzr *vcsCmd, rootDir cmdContext, remoteRepo string) (realRepo string, err error) {
	outb, err := vcsBzr.runOutput(rootDir, "info "+remoteRepo)
	if err != nil {
		return "", err
	}
	out := string(outb)

	// Expect:
	// ...
	//   (branch root|repository branch): <URL>
	// ...

	found := false
	for _, prefix := range []string{"\n  branch root: ", "\n  repository branch: "} {
		i := strings.Index(out, prefix)
		if i >= 0 {
			out = out[i+len(prefix):]
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("unable to parse output of bzr info")
	}

	i := strings.Index(out, "\n")
	if i < 0 {
		return "", fmt.Errorf("unable to parse output of bzr info")
	}
	out = out[:i]
	return strings.TrimSpace(out), nil
}

// vcsSvn describes how to use Subversion.
var vcsSvn = &vcsCmd{
	name: "Subversion",
	cmd:  "svn",

	createCmd:   []string{"checkout -- {repo} {dir}"},
	downloadCmd: []string{"update"},

	// There is no tag command in subversion.
	// The branch information is all in the path names.

	scheme:     []string{"https", "http", "svn", "svn+ssh"},
	pingCmd:    "info -- {scheme}://{repo}",
	remoteRepo: svnRemoteRepo,
}

func svnRemoteRepo(vcsSvn *vcsCmd, rootDir cmdContext) (remoteRepo string, err error) {
	outb, err := vcsSvn.runOutput(rootDir, "info")
	if err != nil {
		return "", err
	}
	out := string(outb)

	// Expect:
	//
	//	 ...
	// 	URL: <URL>
	// 	...
	//
	// Note that we're not using the Repository Root line,
	// because svn allows checking out subtrees.
	// The URL will be the URL of the subtree (what we used with 'svn co')
	// while the Repository Root may be a much higher parent.
	i := strings.Index(out, "\nURL: ")
	if i < 0 {
		return "", fmt.Errorf("unable to parse output of svn info")
	}
	out = out[i+len("\nURL: "):]
	i = strings.Index(out, "\n")
	if i < 0 {
		return "", fmt.Errorf("unable to parse output of svn info")
	}
	out = out[:i]
	return strings.TrimSpace(out), nil
}

// fossilRepoName is the name go get associates with a fossil repository. In the
// real world the file can be named anything.
const fossilRepoName = ".fossil"

// vcsFossil describes how to use Fossil (fossil-scm.org)
var vcsFossil = &vcsCmd{
	name: "Fossil",
	cmd:  "fossil",

	createCmd:   []string{"-go-internal-mkdir {dir} clone -- {repo} " + filepath.Join("{dir}", fossilRepoName), "-go-internal-cd {dir} open .fossil"},
	downloadCmd: []string{"up"},

	tagCmd:         []tagCmd{{"tag ls", `(.*)`}},
	tagSyncCmd:     []string{"up tag:{tag}"},
	tagSyncDefault: []string{"up trunk"},

	scheme:     []string{"https", "http"},
	remoteRepo: fossilRemoteRepo,
}

func fossilRemoteRepo(vcsFossil *vcsCmd, rootDir cmdContext) (remoteRepo string, err error) {
	out, err := vcsFossil.runOutput(rootDir, "remote-url")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (v *vcsCmd) String() string {
	return v.name
}

// run runs the command line cmd in the given directory.
// keyval is a list of key, value pairs. run expands
// instances of {key} in cmd into value, but only after
// splitting cmd into individual arguments.
// If an error occurs, run prints the command line and the
// command's combined stdout+stderr to standard error.
// Otherwise run discards the command's output.
func (v *vcsCmd) run(ctx cmdContext, cmd string, keyval ...string) error {
	_, err := v.run1(ctx, cmd, keyval, true)
	return err
}

// runVerboseOnly is like run but only generates error output to standard error in verbose mode.
func (v *vcsCmd) runVerboseOnly(ctx cmdContext, cmd string, keyval ...string) error {
	_, err := v.run1(ctx, cmd, keyval, false)
	return err
}

// runOutput is like run but returns the output of the command.
func (v *vcsCmd) runOutput(ctx cmdContext, cmd string, keyval ...string) ([]byte, error) {
	return v.run1(ctx, cmd, keyval, true)
}

// run1 is the generalized implementation of run and runOutput.
func (v *vcsCmd) run1(ctx cmdContext, cmdline string, keyval []string, verbose bool) ([]byte, error) {
	dir := ctx.dir
	m := make(map[string]string)
	for i := 0; i < len(keyval); i += 2 {
		m[keyval[i]] = keyval[i+1]
	}
	args := strings.Fields(cmdline)
	for i, arg := range args {
		args[i] = expand(m, arg)
	}

	if len(args) >= 2 && args[0] == "-go-internal-mkdir" {
		var err error
		if filepath.IsAbs(args[1]) {
			err = os.Mkdir(args[1], os.ModePerm)
		} else {
			err = os.Mkdir(filepath.Join(dir, args[1]), os.ModePerm)
		}
		if err != nil {
			return nil, err
		}
		args = args[2:]
	}

	if len(args) >= 2 && args[0] == "-go-internal-cd" {
		if filepath.IsAbs(args[1]) {
			dir = args[1]
		} else {
			dir = filepath.Join(dir, args[1])
		}
		args = args[2:]
	}

	_, err := exec.LookPath(v.cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"go: missing %s command. See https://golang.org/s/gogetcmd\n",
			v.name)
		return nil, err
	}

	cmd := exec.Command(v.cmd, args...)
	// dir defaults to ctx.dir but will be overridden if the command starts with `-go-internal-cd`
	cmd.Dir = dir
	cmd.Env = envForDir(cmd.Dir, os.Environ())

	out, err := cmd.Output()
	if err != nil {
		if verbose {
			fmt.Fprintf(ctx.stderr, "# cd %s; %s %s\n", dir, v.cmd, strings.Join(args, " "))
			if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
				ctx.stderr.Write(ee.Stderr)
			} else {
				fmt.Fprintf(ctx.stderr, "%s\n", err.Error())
			}
		}
	}
	return out, err
}

// ping pings to determine scheme to use.
func (v *vcsCmd) ping(scheme, repo string, stderr io.Writer) error {
	return v.runVerboseOnly(newCmdContext(".", stderr), v.pingCmd, "scheme", scheme, "repo", repo)
}

// create creates a new copy of repo in dir.
// The parent of dir must exist; dir must not.
func (v *vcsCmd) create(dir, repo string, stderr io.Writer) error {
	for _, cmd := range v.createCmd {
		if err := v.run(newCmdContext(".", stderr), cmd, "dir", dir, "repo", repo); err != nil {
			return err
		}
	}
	return nil
}

// download downloads any new changes for the repo in dir.
func (v *vcsCmd) download(dir string, stderr io.Writer) error {
	for _, cmd := range v.downloadCmd {
		if err := v.run(newCmdContext(dir, stderr), cmd); err != nil {
			return err
		}
	}
	return nil
}

// tags returns the list of available tags for the repo in dir.
func (v *vcsCmd) tags(dir string, stderr io.Writer) ([]string, error) {
	var tags []string
	for _, tc := range v.tagCmd {
		out, err := v.runOutput(newCmdContext(dir, stderr), tc.cmd)
		if err != nil {
			return nil, err
		}
		re := regexp.MustCompile(`(?m-s)` + tc.pattern)
		for _, m := range re.FindAllStringSubmatch(string(out), -1) {
			tags = append(tags, m[1])
		}
	}
	return tags, nil
}

// tagSync syncs the repo in dir to the named tag,
// which either is a tag returned by tags or is v.tagDefault.
func (v *vcsCmd) tagSync(dir, tag string, stderr io.Writer) error {
	if v.tagSyncCmd == nil {
		return nil
	}
	cmdCtx := newCmdContext(dir, stderr)
	if tag != "" {
		for _, tc := range v.tagLookupCmd {
			out, err := v.runOutput(cmdCtx, tc.cmd, "tag", tag)
			if err != nil {
				return err
			}
			re := regexp.MustCompile(`(?m-s)` + tc.pattern)
			m := re.FindStringSubmatch(string(out))
			if len(m) > 1 {
				tag = m[1]
				break
			}
		}
	}

	if tag == "" && v.tagSyncDefault != nil {
		for _, cmd := range v.tagSyncDefault {
			if err := v.run(cmdCtx, cmd); err != nil {
				return err
			}
		}
		return nil
	}

	for _, cmd := range v.tagSyncCmd {
		if err := v.run(cmdCtx, cmd, "tag", tag); err != nil {
			return err
		}
	}
	return nil
}

// A vcsPath describes how to convert an import path into a
// version control system and repository name.
type vcsPath struct {
	prefix         string                              // prefix this description applies to
	regexp         *regexp.Regexp                      // compiled pattern for import path
	repo           string                              // repository to use (expand with match of re)
	vcs            string                              // version control system to use (expand with match of re)
	check          func(match map[string]string) error // additional checks
	schemelessRepo bool                                // if true, the repo pattern lacks a scheme
}

// vcsFromDir inspects dir and its parents to determine the
// version control system and code repository to use.
// On return, root is the import path
// corresponding to the root of the repository.
func vcsFromDir(dir, srcRoot string) (vcs *vcsCmd, root string, err error) {
	// Clean and double-check that dir is in (a subdirectory of) srcRoot.
	dir = filepath.Clean(dir)
	srcRoot = filepath.Clean(srcRoot)
	if len(dir) <= len(srcRoot) || dir[len(srcRoot)] != filepath.Separator {
		return nil, "", fmt.Errorf("directory %q is outside source root %q", dir, srcRoot)
	}

	var vcsRet *vcsCmd
	var rootRet string

	origDir := dir
	for len(dir) > len(srcRoot) {
		for _, vcs := range vcsList {
			if _, err := os.Stat(filepath.Join(dir, "."+vcs.cmd)); err == nil {
				root := filepath.ToSlash(dir[len(srcRoot)+1:])
				// Record first VCS we find, but keep looking,
				// to detect mistakes like one kind of VCS inside another.
				if vcsRet == nil {
					vcsRet = vcs
					rootRet = root
					continue
				}
				// Allow .git inside .git, which can arise due to submodules.
				if vcsRet == vcs && vcs.cmd == "git" {
					continue
				}
				// Otherwise, we have one VCS inside a different VCS.
				return nil, "", fmt.Errorf("directory %q uses %s, but parent %q uses %s",
					filepath.Join(srcRoot, rootRet), vcsRet.cmd, filepath.Join(srcRoot, root), vcs.cmd)
			}
		}

		// Move to parent.
		ndir := filepath.Dir(dir)
		if len(ndir) >= len(dir) {
			// Shouldn't happen, but just in case, stop.
			break
		}
		dir = ndir
	}

	if vcsRet != nil {
		return vcsRet, rootRet, nil
	}

	return nil, "", fmt.Errorf("directory %q is not using a known version control system", origDir)
}

// checkNestedVCS checks for an incorrectly-nested VCS-inside-VCS
// situation for dir, checking parents up until srcRoot.
func checkNestedVCS(vcs *vcsCmd, dir, srcRoot string) error {
	if len(dir) <= len(srcRoot) || dir[len(srcRoot)] != filepath.Separator {
		return fmt.Errorf("directory %q is outside source root %q", dir, srcRoot)
	}

	otherDir := dir
	for len(otherDir) > len(srcRoot) {
		for _, otherVCS := range vcsList {
			if _, err := os.Stat(filepath.Join(otherDir, "."+otherVCS.cmd)); err == nil {
				// Allow expected vcs in original dir.
				if otherDir == dir && otherVCS == vcs {
					continue
				}
				// Allow .git inside .git, which can arise due to submodules.
				if otherVCS == vcs && vcs.cmd == "git" {
					continue
				}
				// Otherwise, we have one VCS inside a different VCS.
				return fmt.Errorf("directory %q uses %s, but parent %q uses %s", dir, vcs.cmd, otherDir, otherVCS.cmd)
			}
		}
		// Move to parent.
		newDir := filepath.Dir(otherDir)
		if len(newDir) >= len(otherDir) {
			// Shouldn't happen, but just in case, stop.
			break
		}
		otherDir = newDir
	}

	return nil
}

// repoRoot describes the repository root for a tree of source code.
type repoRoot struct {
	Repo     string // repository URL, including scheme
	Root     string // import path corresponding to root of repo
	IsCustom bool   // defined by served <meta> tags (as opposed to hard-coded pattern)
	VCS      string // vcs type ("mod", "git", ...)

	vcs *vcsCmd // internal: vcs command access
}

func httpPrefix(s string) string {
	for _, prefix := range [...]string{"http:", "https:"} {
		if strings.HasPrefix(s, prefix) {
			return prefix
		}
	}
	return ""
}

// repoRootForImportPath analyzes importPath to determine the
// version control system, and code repository to use.
func repoRootForImportPath(importPath string, security web.SecurityMode, stderr io.Writer) (*repoRoot, error) {
	rr, err := repoRootFromVCSPaths(importPath, security, vcsPaths, stderr)

	// Should have been taken care of above, but make sure.
	if err == nil && strings.Contains(importPath, "...") && strings.Contains(rr.Root, "...") {
		// Do not allow wildcards in the repo root.
		rr = nil
		err = ImportErrorf(importPath, "cannot expand ... in %q", importPath)
	}
	return rr, err
}

var errUnknownSite = errors.New("dynamic lookup required to find mapping")

// repoRootFromVCSPaths attempts to map importPath to a repoRoot
// using the mappings defined in vcsPaths.
func repoRootFromVCSPaths(importPath string, security web.SecurityMode, vcsPaths []*vcsPath, stderr io.Writer) (*repoRoot, error) {
	// A common error is to use https://packagepath because that's what
	// hg and git require. Diagnose this helpfully.
	if prefix := httpPrefix(importPath); prefix != "" {
		// The importPath has been cleaned, so has only one slash. The pattern
		// ignores the slashes; the error message puts them back on the RHS at least.
		return nil, fmt.Errorf("%q not allowed in import path", prefix+"//")
	}
	for _, srv := range vcsPaths {
		if !strings.HasPrefix(importPath, srv.prefix) {
			continue
		}
		m := srv.regexp.FindStringSubmatch(importPath)
		if m == nil {
			if srv.prefix != "" {
				return nil, ImportErrorf(importPath, "invalid %s import path %q", srv.prefix, importPath)
			}
			continue
		}

		// Build map of named subexpression matches for expand.
		match := map[string]string{
			"prefix": srv.prefix,
			"import": importPath,
		}
		for i, name := range srv.regexp.SubexpNames() {
			if name != "" && match[name] == "" {
				match[name] = m[i]
			}
		}
		if srv.vcs != "" {
			match["vcs"] = expand(match, srv.vcs)
		}
		if srv.repo != "" {
			match["repo"] = expand(match, srv.repo)
		}
		if srv.check != nil {
			if err := srv.check(match); err != nil {
				return nil, err
			}
		}
		vcs := vcsByCmd(match["vcs"])
		if vcs == nil {
			return nil, fmt.Errorf("unknown version control system %q", match["vcs"])
		}
		var repoURL string
		if !srv.schemelessRepo {
			repoURL = match["repo"]
		} else {
			scheme := vcs.scheme[0] // default to first scheme
			repo := match["repo"]
			if vcs.pingCmd != "" {
				// If we know how to test schemes, scan to find one.
				for _, s := range vcs.scheme {
					if security == web.SecureOnly && !vcs.isSecureScheme(s) {
						continue
					}
					if vcs.ping(s, repo, stderr) == nil {
						scheme = s
						break
					}
				}
			}
			repoURL = scheme + "://" + repo
		}
		rr := &repoRoot{
			Repo: repoURL,
			Root: match["root"],
			VCS:  vcs.cmd,
			vcs:  vcs,
		}
		return rr, nil
	}
	return nil, errUnknownSite
}

// urlForImportPath returns a partially-populated URL for the given Go import path.
//
// The URL leaves the Scheme field blank so that web.Get will try any scheme
// allowed by the selected security mode.
func urlForImportPath(importPath string) (*urlpkg.URL, error) {
	slash := strings.Index(importPath, "/")
	if slash < 0 {
		slash = len(importPath)
	}
	host, path := importPath[:slash], importPath[slash:]
	if !strings.Contains(host, ".") {
		return nil, errors.New("import path does not begin with hostname")
	}
	if len(path) == 0 {
		path = "/"
	}
	return &urlpkg.URL{Host: host, Path: path, RawQuery: "go-get=1"}, nil
}

// validateRepoRoot returns an error if repoRoot does not seem to be
// a valid URL with scheme.
func validateRepoRoot(repoRoot string) error {
	url, err := urlpkg.Parse(repoRoot)
	if err != nil {
		return err
	}
	if url.Scheme == "" {
		return errors.New("no scheme")
	}
	if url.Scheme == "file" {
		return errors.New("file scheme disallowed")
	}
	return nil
}

type fetchResult struct {
	url *urlpkg.URL
	err error
}

// pathPrefix reports whether sub is a prefix of s,
// only considering entire path components.
func pathPrefix(s, sub string) bool {
	// strings.HasPrefix is necessary but not sufficient.
	if !strings.HasPrefix(s, sub) {
		return false
	}
	// The remainder after the prefix must either be empty or start with a slash.
	rem := s[len(sub):]
	return rem == "" || rem[0] == '/'
}

// expand rewrites s to replace {k} with match[k] for each key k in match.
func expand(match map[string]string, s string) string {
	// We want to replace each match exactly once, and the result of expansion
	// must not depend on the iteration order through the map.
	// A strings.Replacer has exactly the properties we're looking for.
	oldNew := make([]string, 0, 2*len(match))
	for k, v := range match {
		oldNew = append(oldNew, "{"+k+"}", v)
	}
	return strings.NewReplacer(oldNew...).Replace(s)
}

// vcsPaths defines the meaning of import paths referring to
// commonly-used VCS hosting sites (github.com/user/dir)
// and import paths referring to a fully-qualified importPath
// containing a VCS type (foo.com/repo.git/dir)
var vcsPaths = []*vcsPath{
	// Github
	{
		prefix: "github.com/",
		regexp: regexp.MustCompile(`^(?P<root>github\.com/[A-Za-z0-9_.\-]+/[A-Za-z0-9_.\-]+)(/[\p{L}0-9_.\-]+)*$`),
		vcs:    "git",
		repo:   "https://{root}",
		check:  noVCSSuffix,
	},

	// Gitlab
	{
		prefix: "gitlab.com/",
		regexp: regexp.MustCompile(`^(?P<root>gitlab\.com/[A-Za-z0-9_.\-]+/[A-Za-z0-9_.\-]+)(/[\p{L}0-9_.\-]+)*$`),
		vcs:    "git",
		repo:   "https://{root}",
		check:  noVCSSuffix,
	},

	// Bitbucket
	{
		prefix: "bitbucket.org/",
		regexp: regexp.MustCompile(`^(?P<root>bitbucket\.org/(?P<bitname>[A-Za-z0-9_.\-]+/[A-Za-z0-9_.\-]+))(/[A-Za-z0-9_.\-]+)*$`),
		repo:   "https://{root}",
		check:  bitbucketVCS,
	},

	// IBM DevOps Services (JazzHub)
	{
		prefix: "hub.jazz.net/git/",
		regexp: regexp.MustCompile(`^(?P<root>hub\.jazz\.net/git/[a-z0-9]+/[A-Za-z0-9_.\-]+)(/[A-Za-z0-9_.\-]+)*$`),
		vcs:    "git",
		repo:   "https://{root}",
		check:  noVCSSuffix,
	},

	// Git at Apache
	{
		prefix: "git.apache.org/",
		regexp: regexp.MustCompile(`^(?P<root>git\.apache\.org/[a-z0-9_.\-]+\.git)(/[A-Za-z0-9_.\-]+)*$`),
		vcs:    "git",
		repo:   "https://{root}",
	},

	// Git at OpenStack
	{
		prefix: "git.openstack.org/",
		regexp: regexp.MustCompile(`^(?P<root>git\.openstack\.org/[A-Za-z0-9_.\-]+/[A-Za-z0-9_.\-]+)(\.git)?(/[A-Za-z0-9_.\-]+)*$`),
		vcs:    "git",
		repo:   "https://{root}",
	},

	// chiselapp.com for fossil
	{
		prefix: "chiselapp.com/",
		regexp: regexp.MustCompile(`^(?P<root>chiselapp\.com/user/[A-Za-z0-9]+/repository/[A-Za-z0-9_.\-]+)$`),
		vcs:    "fossil",
		repo:   "https://{root}",
	},

	// General syntax for any server.
	// Must be last.
	{
		regexp:         regexp.MustCompile(`(?P<root>(?P<repo>([a-z0-9.\-]+\.)+[a-z0-9.\-]+(:[0-9]+)?(/~?[A-Za-z0-9_.\-]+)+?)\.(?P<vcs>bzr|fossil|git|hg|svn))(/~?[A-Za-z0-9_.\-]+)*$`),
		schemelessRepo: true,
	},
}

// noVCSSuffix checks that the repository name does not
// end in .foo for any version control system foo.
// The usual culprit is ".git".
func noVCSSuffix(match map[string]string) error {
	repo := match["repo"]
	for _, vcs := range vcsList {
		if strings.HasSuffix(repo, "."+vcs.cmd) {
			return fmt.Errorf("invalid version control suffix in %s path", match["prefix"])
		}
	}
	return nil
}

// bitbucketVCS determines the version control system for a
// Bitbucket repository, by using the Bitbucket API.
func bitbucketVCS(match map[string]string) error {
	if err := noVCSSuffix(match); err != nil {
		return err
	}

	var resp struct {
		SCM string `json:"scm"`
	}
	url := &urlpkg.URL{
		Scheme:   "https",
		Host:     "api.bitbucket.org",
		Path:     expand(match, "/2.0/repositories/{bitname}"),
		RawQuery: "fields=scm",
	}
	data, err := web.GetBytes(url)
	if err != nil {
		if httpErr, ok := err.(*web.HTTPError); ok && httpErr.StatusCode == 403 {
			// this may be a private repository. If so, attempt to determine which
			// VCS it uses. See issue 5375.
			root := match["root"]
			for _, vcs := range []string{"git", "hg"} {
				if vcsByCmd(vcs).ping("https", root, os.Stderr) == nil {
					resp.SCM = vcs
					break
				}
			}
		}

		if resp.SCM == "" {
			return err
		}
	} else {
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("decoding %s: %v", url, err)
		}
	}

	if vcsByCmd(resp.SCM) != nil {
		match["vcs"] = resp.SCM
		if resp.SCM == "git" {
			match["repo"] += ".git"
		}
		return nil
	}

	return fmt.Errorf("unable to detect version control system for bitbucket.org/ path")
}
