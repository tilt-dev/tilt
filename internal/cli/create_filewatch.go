package cli

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// A human-friendly CLI for creating file watches.
//
// (as opposed to the machine-friendly CLIs of create -f or apply -f)
type createFileWatchCmd struct {
	helper *createHelper
	cmd    *cobra.Command

	ignoreValues []string
}

var _ tiltCmd = &createFileWatchCmd{}

func newCreateFileWatchCmd() *createFileWatchCmd {
	helper := newCreateHelper()
	return &createFileWatchCmd{
		helper: helper,
	}
}

func (c *createFileWatchCmd) name() model.TiltSubcommand { return "filewatch" }

func (c *createFileWatchCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "filewatch [NAME] [PATHS] --ignore [IGNORES]",
		DisableFlagsInUseLine: true,
		Short:                 "Create a filewatch in a running tilt session",
		Long: `Create a FileWatch in a running tilt session.

To create a file watch, first supply the name of the
watch so you can reference it later. Then supply the paths
to watch. All paths will be watched recursively.

On its own, a FileWatch is an object that watches a set
of files, and updates its status field with the most recent
file changed.

A FileWatch is intended to combine with other Tilt objects to
trigger events when a file changes.
`,
		Aliases: []string{"fw"},
		Args:    cobra.MinimumNArgs(2),
		Example: `tilt create fw src-and-web src web --ignore=web/node_modules`,
	}

	cmd.Flags().StringSliceVar(&c.ignoreValues, "ignore", nil,
		"Patterns to ignore. Supports same syntax as .dockerignore. Paths are relative to the current directory.")

	c.helper.addFlags(cmd)
	c.cmd = cmd

	return cmd
}

func (c *createFileWatchCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.create-filewatch", cmdTags.AsMap())
	defer a.Flush(time.Second)

	err := c.helper.interpretFlags(ctx)
	if err != nil {
		return err
	}

	fw, err := c.object(args)
	if err != nil {
		return err
	}

	return c.helper.create(ctx, fw)
}

// Interprets the flags specified on the commandline to the FileWatch to create.
func (c *createFileWatchCmd) object(args []string) (*v1alpha1.FileWatch, error) {
	name := args[0]
	pathArgs := args[1:]

	paths, err := c.paths(pathArgs)
	if err != nil {
		return nil, err
	}

	ignores, err := c.ignores()
	if err != nil {
		return nil, err
	}

	fw := v1alpha1.FileWatch{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.FileWatchSpec{
			WatchedPaths: paths,
			Ignores:      ignores,
		},
	}
	return &fw, nil
}

// Interprets the paths specified on the commandline.
func (c *createFileWatchCmd) paths(pathArgs []string) ([]string, error) {
	result := []string{}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	for _, path := range pathArgs {
		if filepath.IsAbs(path) {
			result = append(result, path)
		} else {
			result = append(result, filepath.Join(cwd, path))
		}
	}
	return result, nil
}

// Interprets the ignores specified on the commandline.
func (c *createFileWatchCmd) ignores() ([]v1alpha1.IgnoreDef, error) {
	result := v1alpha1.IgnoreDef{}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	result.BasePath = cwd
	result.Patterns = append([]string{}, c.ignoreValues...)
	return []v1alpha1.IgnoreDef{result}, nil
}
