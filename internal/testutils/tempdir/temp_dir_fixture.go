package tempdir

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type TempDirFixture struct {
	t      testing.TB
	dir    string
	oldDir string
}

// everything not listed in this character class will get replaced by _, so that it's a safe filename
var sanitizeForFilenameRe = regexp.MustCompile("[^a-zA-Z0-9.]")

func SanitizeFileName(name string) string {
	return sanitizeForFilenameRe.ReplaceAllString(name, "_")
}

func NewTempDirFixture(t testing.TB) *TempDirFixture {
	dir := t.TempDir()

	dir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)

	f := &TempDirFixture{
		t:   t,
		dir: dir,
	}
	t.Cleanup(f.tearDown)
	return f
}

func (f *TempDirFixture) T() testing.TB {
	return f.t
}

func (f *TempDirFixture) Path() string {
	return f.dir
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
	err = os.WriteFile(fullPath, []byte(contents), os.FileMode(0777))
	if err != nil {
		f.t.Fatal(err)
	}
	return fullPath
}

// Returns the full path to the file written.
func (f *TempDirFixture) CopyFile(originalPath, newPath string) {
	contents, err := os.ReadFile(originalPath)
	if err != nil {
		f.t.Fatal(err)
	}
	f.WriteFile(newPath, string(contents))
}

// Read the file.
func (f *TempDirFixture) ReadFile(path string) string {
	fullPath := f.JoinPath(path)
	contents, err := os.ReadFile(fullPath)
	if err != nil {
		f.t.Fatal(err)
	}
	return string(contents)
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
	return os.CreateTemp(f.dir, prefix)
}

func (f *TempDirFixture) TempDir(prefix string) string {
	name, err := os.MkdirTemp(f.dir, prefix)
	if err != nil {
		f.t.Fatal(err)
	}
	return name
}

func (f *TempDirFixture) tearDown() {
	if f.oldDir != "" {
		err := os.Chdir(f.oldDir)
		if err != nil {
			f.t.Fatal(err)
		}
	}
}
