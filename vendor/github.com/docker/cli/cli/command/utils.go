// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.19

package command

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/cli/cli/streams"
	"github.com/docker/docker/api/types/filters"
	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/versions"
	"github.com/moby/sys/sequential"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

// CopyToFile writes the content of the reader to the specified file
func CopyToFile(outfile string, r io.Reader) error {
	// We use sequential file access here to avoid depleting the standby list
	// on Windows. On Linux, this is a call directly to os.CreateTemp
	tmpFile, err := sequential.CreateTemp(filepath.Dir(outfile), ".docker_temp_")
	if err != nil {
		return err
	}

	tmpPath := tmpFile.Name()

	_, err = io.Copy(tmpFile, r)
	tmpFile.Close()

	if err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err = os.Rename(tmpPath, outfile); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}

// capitalizeFirst capitalizes the first character of string
func capitalizeFirst(s string) string {
	switch l := len(s); l {
	case 0:
		return s
	case 1:
		return strings.ToLower(s)
	default:
		return strings.ToUpper(string(s[0])) + strings.ToLower(s[1:])
	}
}

// PrettyPrint outputs arbitrary data for human formatted output by uppercasing the first letter.
func PrettyPrint(i any) string {
	switch t := i.(type) {
	case nil:
		return "None"
	case string:
		return capitalizeFirst(t)
	default:
		return capitalizeFirst(fmt.Sprintf("%s", t))
	}
}

// PromptForConfirmation requests and checks confirmation from user.
// This will display the provided message followed by ' [y/N] '. If
// the user input 'y' or 'Y' it returns true other false.  If no
// message is provided "Are you sure you want to proceed? [y/N] "
// will be used instead.
func PromptForConfirmation(ins io.Reader, outs io.Writer, message string) bool {
	if message == "" {
		message = "Are you sure you want to proceed?"
	}
	message += " [y/N] "

	_, _ = fmt.Fprint(outs, message)

	// On Windows, force the use of the regular OS stdin stream.
	if runtime.GOOS == "windows" {
		ins = streams.NewIn(os.Stdin)
	}

	reader := bufio.NewReader(ins)
	answer, _, _ := reader.ReadLine()
	return strings.ToLower(string(answer)) == "y"
}

// PruneFilters returns consolidated prune filters obtained from config.json and cli
func PruneFilters(dockerCli Cli, pruneFilters filters.Args) filters.Args {
	if dockerCli.ConfigFile() == nil {
		return pruneFilters
	}
	for _, f := range dockerCli.ConfigFile().PruneFilters {
		k, v, ok := strings.Cut(f, "=")
		if !ok {
			continue
		}
		if k == "label" {
			// CLI label filter supersede config.json.
			// If CLI label filter conflict with config.json,
			// skip adding label! filter in config.json.
			if pruneFilters.Contains("label!") && pruneFilters.ExactMatch("label!", v) {
				continue
			}
		} else if k == "label!" {
			// CLI label! filter supersede config.json.
			// If CLI label! filter conflict with config.json,
			// skip adding label filter in config.json.
			if pruneFilters.Contains("label") && pruneFilters.ExactMatch("label", v) {
				continue
			}
		}
		pruneFilters.Add(k, v)
	}

	return pruneFilters
}

// AddPlatformFlag adds `platform` to a set of flags for API version 1.32 and later.
func AddPlatformFlag(flags *pflag.FlagSet, target *string) {
	flags.StringVar(target, "platform", os.Getenv("DOCKER_DEFAULT_PLATFORM"), "Set platform if server is multi-platform capable")
	flags.SetAnnotation("platform", "version", []string{"1.32"})
}

// ValidateOutputPath validates the output paths of the `export` and `save` commands.
func ValidateOutputPath(path string) error {
	dir := filepath.Dir(filepath.Clean(path))
	if dir != "" && dir != "." {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return errors.Errorf("invalid output path: directory %q does not exist", dir)
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
			return errors.Wrapf(err, fmt.Sprintf("invalid output path: %q must be a directory or a regular file", path))
		}
	}
	return nil
}

// ValidateOutputPathFileMode validates the output paths of the `cp` command and serves as a
// helper to `ValidateOutputPath`
func ValidateOutputPathFileMode(fileMode os.FileMode) error {
	switch {
	case fileMode&os.ModeDevice != 0:
		return errors.New("got a device")
	case fileMode&os.ModeIrregular != 0:
		return errors.New("got an irregular file")
	}
	return nil
}

func stringSliceIndex(s, subs []string) int {
	j := 0
	if len(subs) > 0 {
		for i, x := range s {
			if j < len(subs) && subs[j] == x {
				j++
			} else {
				j = 0
			}
			if len(subs) == j {
				return i + 1 - j
			}
		}
	}
	return -1
}

// StringSliceReplaceAt replaces the sub-slice find, with the sub-slice replace, in the string
// slice s, returning a new slice and a boolean indicating if the replacement happened.
// requireIdx is the index at which old needs to be found at (or -1 to disregard that).
func StringSliceReplaceAt(s, find, replace []string, requireIndex int) ([]string, bool) {
	idx := stringSliceIndex(s, find)
	if (requireIndex != -1 && requireIndex != idx) || idx == -1 {
		return s, false
	}
	out := append([]string{}, s[:idx]...)
	out = append(out, replace...)
	out = append(out, s[idx+len(find):]...)
	return out, true
}

// ValidateMountWithAPIVersion validates a mount with the server API version.
func ValidateMountWithAPIVersion(m mounttypes.Mount, serverAPIVersion string) error {
	if m.BindOptions != nil {
		if m.BindOptions.NonRecursive && versions.LessThan(serverAPIVersion, "1.40") {
			return errors.Errorf("bind-recursive=disabled requires API v1.40 or later")
		}
		// ReadOnlyNonRecursive can be safely ignored when API < 1.44
		if m.BindOptions.ReadOnlyForceRecursive && versions.LessThan(serverAPIVersion, "1.44") {
			return errors.Errorf("bind-recursive=readonly requires API v1.44 or later")
		}
	}
	return nil
}
