// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.24

package command

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/cli/cli/config"
	"github.com/moby/moby/client"
)

// PruneFilters merges prune filters specified in config.json with those specified
// as command-line flags. It returns a deep copy of filters to prevent mutating
// the original.
//
// CLI label filters have precedence over those specified in config.json. If a
// label filter specified as flag conflicts with a label defined in config.json
// (i.e., "label=some-value" conflicts with "label!=some-value", and vice versa),
// then the filter defined in config.json is omitted.
func PruneFilters(dockerCLI config.Provider, filters client.Filters) client.Filters {
	pruneFilters := filters.Clone()

	cfg := dockerCLI.ConfigFile()
	if cfg == nil {
		return pruneFilters
	}

	// Merge filters provided through the CLI with default filters defined
	// in the CLI-configfile.
	for _, f := range cfg.PruneFilters {
		k, v, ok := strings.Cut(f, "=")
		if !ok {
			continue
		}
		switch k {
		case "label":
			// "label != some-value" conflicts with "label = some-value"
			if pruneFilters["label!"][v] {
				continue
			}
			pruneFilters.Add(k, v)
		case "label!":
			// "label != some-value" conflicts with "label = some-value"
			if pruneFilters["label"][v] {
				continue
			}
			pruneFilters.Add(k, v)
		default:
			pruneFilters.Add(k, v)
		}
	}

	return pruneFilters
}

// ValidateOutputPath validates the output paths of the "docker cp" command.
func ValidateOutputPath(path string) error {
	dir := filepath.Dir(filepath.Clean(path))
	if dir != "" && dir != "." {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("invalid output path: directory %q does not exist", dir)
		}
	}
	// check whether `path` points to a regular file
	// (if the path exists and doesn't point to a directory)
	if fileInfo, err := os.Stat(path); !os.IsNotExist(err) {
		if err != nil {
			return err
		}

		if fileInfo.Mode().IsDir() || fileInfo.Mode().IsRegular() {
			return nil
		}

		if err := ValidateOutputPathFileMode(fileInfo.Mode()); err != nil {
			return fmt.Errorf("invalid output path: %q must be a directory or a regular file: %w", path, err)
		}
	}
	return nil
}

// ValidateOutputPathFileMode validates the output paths of the "docker cp" command
// and serves as a helper to [ValidateOutputPath]
func ValidateOutputPathFileMode(fileMode os.FileMode) error {
	switch {
	case fileMode&os.ModeDevice != 0:
		return errors.New("got a device")
	case fileMode&os.ModeIrregular != 0:
		return errors.New("got an irregular file")
	}
	return nil
}

func invalidParameter(err error) error {
	return invalidParameterErr{err}
}

type invalidParameterErr struct{ error }

func (invalidParameterErr) InvalidParameter() {}

func notFound(err error) error {
	return notFoundErr{err}
}

type notFoundErr struct{ error }

func (notFoundErr) NotFound() {}
func (e notFoundErr) Unwrap() error {
	return e.error
}
