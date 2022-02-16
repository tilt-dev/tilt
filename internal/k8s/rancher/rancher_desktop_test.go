package rancher

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/pkg/logger"
)

func TestDetermineContainerRuntime_Docker(t *testing.T) {
	contents := `{"version":4,"kubernetes":{"containerEngine":"moby"}}`
	setConfigContents(t, contents)

	ctx := logger.WithLogger(context.Background(), logger.NewTestLogger(os.Stdout))
	runtime := DetermineContainerRuntime(ctx)
	require.Equal(t, ContainerRuntimeDocker, runtime)
}

func TestDetermineContainerRuntime_Containerd(t *testing.T) {
	contents := `{"version":4,"kubernetes":{"containerEngine":"containerd"}}`
	setConfigContents(t, contents)

	ctx := logger.WithLogger(context.Background(), logger.NewTestLogger(os.Stdout))
	runtime := DetermineContainerRuntime(ctx)
	require.Equal(t, ContainerRuntimeContainerd, runtime)
}

func TestDetermineContainerRuntime_Unknown(t *testing.T) {
	tcs := map[string]string{
		"FieldUnsupported": `{"version":5,"kubernetes":{"containerEngine":"oh-no"}}`,
		"FieldNull":        `{"version":4,"kubernetes":{"containerEngine":null}}`,
		"FieldBlank":       `{"version":4,"kubernetes":{"containerEngine":""}}`,
		"FieldMissing":     `{"version":4,"kubernetes":{}}`,
		"ParentMissing":    `{"version":4}`,
		"ParentNull":       `{"version":4,"kubernetes":null}`,
		"ConfigEmpty":      `{}`,
		"ConfigNull":       `null`,
		"ConfigBlank":      ``,
		// N.B. this is missing a trailing } to make it invalid JSON
		"ConfigInvalidJSON": `{"version":4,"kubernetes":{"containerEngine":"containerd"}`,
	}

	ctx := logger.WithLogger(context.Background(), logger.NewTestLogger(os.Stdout))
	for name, cfg := range tcs {
		t.Run(name, func(t *testing.T) {
			setConfigContents(t, cfg)
			runtime := DetermineContainerRuntime(ctx)
			require.Equal(t, ContainerRuntimeUnknown, runtime)
		})
	}

}

func TestDetermineContainerRuntime_ConfigReadError(t *testing.T) {
	setConfigError(t, errors.New("not found"))

	ctx := logger.WithLogger(context.Background(), logger.NewTestLogger(os.Stdout))
	runtime := DetermineContainerRuntime(ctx)
	require.Equal(t, ContainerRuntimeUnknown, runtime)
}

func setConfigContents(t testing.TB, contents string) {
	replaceOpenFileForTest(t, func(name string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(contents)), nil
	})
}

func setConfigError(t testing.TB, err error) {
	replaceOpenFileForTest(t, func(name string) (io.ReadCloser, error) {
		return nil, err
	})
}

func replaceOpenFileForTest(t testing.TB, openFunc openFileFunc) {
	origOpenFile := openFile
	t.Cleanup(func() {
		openFile = origOpenFile
	})
	openFile = openFunc
}
