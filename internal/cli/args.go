package cli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kballard/go-shellquote"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/util/editor"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type argsCmd struct {
	streams genericclioptions.IOStreams
	clear   bool
}

func newArgsCmd(streams genericclioptions.IOStreams) *argsCmd {
	return &argsCmd{
		streams: streams,
	}
}

func (c *argsCmd) name() model.TiltSubcommand { return "args" }

func (c *argsCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "args [<flags>] [-- <Tiltfile args>]",
		DisableFlagsInUseLine: true,
		Short:                 "Changes the Tiltfile args in use by a running Tilt",
		Long: `Changes the Tiltfile args in use by a running Tilt.

The "args" can be used to specify what resources Tilt enables. They can also 
be used to define custom key-value pairs that can be accessed in a Tiltfile
by calling "config.parse()".

If no args are explicitly specified, (i.e., just "tilt args"), it opens the current args for editing in
the editor defined by your TILT_EDITOR or EDITOR environment variables, or fall back to
an OS-appropriate default.

Note that Tiltfile arguments do not affect built-in Tilt args (i.e., the things that show up in "tilt up --help", such as "--legacy", "--port"), and they
are defined after built-in args, following a "--".`,
		Example: `# Set new args
tilt args frontend_service backend_service -- --debug on

# Edit the current args
tilt args

# Use an alternative editor
EDITOR=nano tilt args`,
	}

	addConnectServerFlags(cmd)
	cmd.Flags().BoolVar(&c.clear, "clear", false, "Clear the Tiltfile args, as if you'd run tilt with no args")

	return cmd
}

func parseEditResult(b []byte) ([]string, error) {
	sc := bufio.NewScanner(bytes.NewReader(b))
	var argsLine *string
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		if argsLine != nil {
			return nil, errors.New("cannot have multiple non-comment lines")
		}
		s := line
		argsLine = &s
	}
	if argsLine == nil {
		return nil, errors.New("must have exactly one non-comment line, found zero. If you want to clear the args, use `tilt args --clear`")
	}
	args, err := shellquote.Split(*argsLine)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing %q", string(b))
	}

	return args, nil
}

func (c *argsCmd) run(ctx context.Context, args []string) error {
	ctx = logger.WithLogger(ctx, logger.NewLogger(logger.Get(ctx).Level(), c.streams.ErrOut))

	ctrlclient, err := newClient(ctx)
	if err != nil {
		return err
	}

	var tf v1alpha1.Tiltfile
	err = ctrlclient.Get(ctx, types.NamespacedName{Name: model.MainTiltfileManifestName.String()}, &tf)
	if err != nil {
		return err
	}

	tags := make(engineanalytics.CmdTags)
	if c.clear {
		if len(args) != 0 {
			return errors.New("--clear cannot be specified with other values")
		}
		args = nil
		tags["clear"] = "true"
	} else if len(args) == 0 {
		input := fmt.Sprintf("# edit args for the running Tilt here\n%s\n", shellquote.Join(tf.Spec.Args...))
		e := editor.NewDefaultEditor([]string{"TILT_EDITOR", "EDITOR"})
		b, _, err := e.LaunchTempFile("", "", strings.NewReader(input))
		if err != nil {
			return err
		}

		args, err = parseEditResult(b)
		if err != nil {
			return err
		}
		tags["edit"] = "true"
	} else {
		tags["set"] = "true"
	}

	a := analytics.Get(ctx)
	a.Incr("cmd.args", tags.AsMap())
	defer a.Flush(time.Second)

	if sliceutils.StringSliceEquals(tf.Spec.Args, args) {
		logger.Get(ctx).Infof("Tilt is already running with those args -- no action taken")
		return nil
	}
	tf.Spec.Args = args

	err = ctrlclient.Update(ctx, &tf)
	if err != nil {
		return err
	}

	logger.Get(ctx).Infof("Changed config args for Tilt running at %s to %v", apiHost(), args)

	return nil
}
