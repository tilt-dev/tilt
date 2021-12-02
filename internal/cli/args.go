package cli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/cmd/util/editor"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

type argsCmd struct {
	clear bool
}

func newArgsCmd() *argsCmd {
	return &argsCmd{}
}

func (c *argsCmd) name() model.TiltSubcommand { return "args" }

func (c *argsCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "args [<flags>] [-- <Tiltfile args>]",
		DisableFlagsInUseLine: true,
		Short:                 "Changes the Tiltfile args in use by a running Tilt",
		Long: `Changes the Tiltfile args in use by a running Tilt.
If no args are specified, (i.e., just "tilt args"), opens the current args for editing in
the editor defined by your TILT_EDITOR or EDITOR environment variables, or fall back to
an OS-appropriate default.

The editor can be skipped by providing new args on the command line, e.g.: "tilt args svc1 svc2".

# Edit the args
tilt args

# Use an alternate editor
EDITOR=nano tilt args

# skip the editor
# note: "--" here indicates the end of the tilt args and the start of the tiltfile args
tilt args -- --foo=bar frontend backend

Note that this does not affect built-in Tilt args (e.g. --hud, --host), but rather the extra args that come after,
i.e., those specifying which resources to run and/or handled by a Tiltfile calling config.parse.
`,
	}

	addConnectServerFlags(cmd)
	cmd.Flags().BoolVar(&c.clear, "clear", false, "Clear the Tiltfile args, as if you'd run tilt with no args")

	return cmd
}

func newClient(ctx context.Context) (client.Client, error) {
	getter, err := wireClientGetter(ctx)
	if err != nil {
		return nil, err
	}

	cfg, err := getter.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	ctrlclient, err := client.New(cfg, client.Options{Scheme: v1alpha1.NewScheme()})
	if err != nil {
		return nil, err
	}

	return ctrlclient, err
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
	ctrlclient, err := newClient(ctx)
	if err != nil {
		return err
	}

	var tf v1alpha1.Tiltfile
	err = ctrlclient.Get(ctx, types.NamespacedName{Name: model.MainTiltfileManifestName.String()}, &tf)
	if err != nil {
		return err
	}

	if c.clear {
		if len(args) != 0 {
			return errors.New("--clear cannot be specified with other values")
		}
		args = nil
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
	}

	tf.Spec.Args = args

	err = ctrlclient.Update(ctx, &tf)
	if err != nil {
		return err
	}

	fmt.Printf("changed config args for Tilt running at %s to %v\n", apiHost(), args)

	return nil
}
