package tempdir

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/tilt-dev/wmclient/pkg/os/temp"
)

type TempDirFixture struct {
	t      testing.TB
	dir    *temp.TempDir
	oldDir string
}

// everything not listed in this character class will get replaced by _, so that it's a safe filename
var sanitizeForFilenameRe = regexp.MustCompile("[^a-zA-Z0-9.]")

func SanitizeFileName(name string) string {
	return sanitizeForFilenameRe.ReplaceAllString(name, "_")
}

func NewTempDirFixture(t testing.TB) *TempDirFixture {
	dir, err := temp.NewDir(SanitizeFileName(t.Name()))
	if err != nil {
		t.Fatalf("Error making temp dir: %v", err)
	}

	return &TempDirFixture{
		t:   t,
		dir: dir,
	}
}

func (f *TempDirFixture) T() testing.TB {
	return f.t
}

func (f *TempDirFixture) Path() string {
	return f.dir.Path()
}

func (f *TempDirFixture) Chdir() {
	cwd, err := os.Getwd()
	if err != nil {
		f.t.Fatal(err)
	}

	f.oldDir = cwd

	err = os.Chdir(f.Path())
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *TempDirFixture) JoinPath(path ...string) string {
	p := []string{}
	isAbs := len(path) > 0 && filepath.IsAbs(path[0])
	if isAbs {
		if !strings.HasPrefix(path[0], f.Path()) {
			f.t.Fatalf("Path outside fixture tempdir are forbidden: %s", path[0])
		}
	} else {
		p = append(p, f.Path())
	}

	p = append(p, path...)
	return filepath.Join(p...)
}

func (f *TempDirFixture) JoinPaths(paths []string) []string {
	joined := make([]string, len(paths))
	for i, p := range paths {
		joined[i] = f.JoinPath(p)
	}
	return joined
}

// Returns the full path to the file written.
func (f *TempDirFixture) WriteFile(path string, contents string) string {
	fullPath := f.JoinPath(path)
	base := filepath.Dir(fullPath)
	err := os.MkdirAll(base, os.FileMode(0777))
	if err != nil {
		f.t.Fatal(err)
	}
	err = ioutil.WriteFile(fullPath, []byte(contents), os.FileMode(0777))
	if err != nil {
		f.t.Fatal(err)
	}
	return fullPath
}

// Returns the full path to the file written.
func (f *TempDirFixture) CopyFile(originalPath, newPath string) {
	contents, err := ioutil.ReadFile(originalPath)
	if err != nil {
		f.t.Fatal(err)
	}
	f.WriteFile(newPath, string(contents))
}

func (f *TempDirFixture) WriteSymlink(linkContents, destPath string) {
	fullDestPath := f.JoinPath(destPath)
	err := os.MkdirAll(filepath.Dir(fullDestPath), os.FileMode(0777))
	if err != nil {
		f.t.Fatal(err)
	}
	err = os.Symlink(linkContents, fullDestPath)
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *TempDirFixture) MkdirAll(path string) {
	fullPath := f.JoinPath(path)
	err := os.MkdirAll(fullPath, os.FileMode(0777))
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *TempDirFixture) TouchFiles(paths []string) {
	for _, p := range paths {
		f.WriteFile(p, "")
	}
}

func (f *TempDirFixture) Rm(pathInRepo string) {
	fullPath := f.JoinPath(pathInRepo)
	err := os.RemoveAll(fullPath)
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *TempDirFixture) NewFile(prefix string) (*os.File, error) {
	return ioutil.TempFile(f.dir.Path(), prefix)
}

func (f *TempDirFixture) TempDir(prefix string) string {
	name, err := ioutil.TempDir(f.dir.Path(), prefix)
	if err != nil {
		f.t.Fatal(err)
	}
	return name
}

func (f *TempDirFixture) TearDown() {
	if f.oldDir != "" {
		err := os.Chdir(f.oldDir)
		if err != nil {
			f.t.Fatal(err)
		}
	}

	err := f.dir.TearDown()
	if err != nil && runtime.GOOS == "windows" &&
		(strings.Contains(err.Error(), "The process cannot access the file") ||
			strings.Contains(err.Error(), "Access is denied")) {
		// NOTE(nick): I'm not convinced that this is a real problem.
		// I think it might just be clean up of file notification I/O.
	} else if err != nil {
		f.t.Fatal(err)
	}
}
