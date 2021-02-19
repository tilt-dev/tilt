package tiltextension

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const testLibText = `
def printFoo():
  print("foo")
`

func dirWithTiltfile(t *testing.T, contents string) string {
	dir, err := ioutil.TempDir("", "fakeFetcher")
	require.NoError(t, err)

	err = ioutil.WriteFile(filepath.Join(dir, "Tiltfile"), []byte(contents), os.FileMode(0644))
	require.NoError(t, err)

	return dir
}

type fakeFetcher struct {
	t *testing.T
}

func (f *fakeFetcher) Fetch(ctx context.Context, moduleName string) (ModuleContents, error) {
	if moduleName != "fetchable" {
		return ModuleContents{}, fmt.Errorf("module %s can't be fetched because... reasons", moduleName)
	}

	return ModuleContents{
		Name: "fetchable",
		Dir:  dirWithTiltfile(f.t, testLibText),
	}, nil
}

func (f *fakeFetcher) CleanUp() error {
	return nil
}

func NewFakeExtension(t *testing.T, tmpdir string) *Extension {
	return NewExtension(
		&fakeFetcher{t: t},
		NewLocalStore(tmpdir),
	)
}
