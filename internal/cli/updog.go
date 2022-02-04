package cli

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/cli/visitor"
	"github.com/tilt-dev/tilt/internal/controllers"
	"github.com/tilt-dev/tilt/internal/engine"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/internal/hud"
	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type updogCmd struct {
	*genericclioptions.FileNameFlags
	genericclioptions.IOStreams

	Filenames []string
}

var _ tiltCmd = &updogCmd{}

func newUpdogCmd(streams genericclioptions.IOStreams) *updogCmd {
	c := &updogCmd{
		IOStreams: streams,
	}
	c.FileNameFlags = &genericclioptions.FileNameFlags{Filenames: &c.Filenames}
	return c
}

func (c *updogCmd) name() model.TiltSubcommand { return "updog" }

func (c *updogCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "updog",
		Short: "Demo app that runs the Tilt apiserver, applies the config, and streams logs until ctrl-c",
		Long: `Demo app that runs the Tilt apiserver, applies the config, and streams logs until ctrl-c.

Starts a "blank slate" version of Tilt that only uses the new Tilt API.

Doesn't execute the Tiltfile.

For use in demos where we want to bring up the Tilt apiserver with
a set of configs. Here's a youtube video showing it in action:

https://www.youtube.com/watch?v=dQw4w9WgXcQ

See https://github.com/tilt-dev/tilt/tree/master/internal/cli/updog-examples
for example configs.
`,
		Example: "tilt alpha updog -f config.yaml",
	}

	addStartServerFlags(cmd)
	c.FileNameFlags.AddFlags(cmd.Flags())

	return cmd
}

func (c *updogCmd) run(ctx context.Context, args []string) error {

	a := analytics.Get(ctx)

	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.updog", cmdTags.AsMap())
	defer a.Flush(time.Second)

	if len(c.Filenames) == 0 {
		return fmt.Errorf("Expected source files with -f")
	}

	visitors, err := visitor.FromStrings(c.Filenames, c.In)
	if err != nil {
		return fmt.Errorf("Parsing inputs: %v", err)
	}

	objects, err := visitor.DecodeAll(v1alpha1.NewScheme(), visitors)
	if err != nil {
		return fmt.Errorf("Decoding inputs: %v", err)
	}

	clientObjects, err := convertToClientObjects(objects)
	if err != nil {
		return err
	}

	deferred := logger.NewDeferredLogger(ctx)
	ctx = redirectLogs(ctx, deferred)

	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

	// Force web-mode to prod
	webModeFlag = model.ProdWebMode

	log.Print("Tilt updog " + buildStamp())

	deps, err := wireCmdUpdog(ctx, a, nil, "updog", clientObjects)
	if err != nil {
		deferred.SetOutput(deferred.Original())
		return err
	}

	l := store.NewLogActionLogger(ctx, deps.Upper.Dispatch)
	deferred.SetOutput(l)
	ctx = redirectLogs(ctx, l)

	// A lot of these parameters don't matter because we don't have any
	// controllers registered.
	err = deps.Upper.Start(ctx, args, deps.TiltBuild,
		"Tiltfile", store.TerminalModeStream, a.UserOpt(), deps.Token,
		string(deps.CloudAddress))
	if err != context.Canceled {
		return err
	} else {
		return nil
	}
}

// Once the API server starts, create all the objects that were fed in
// on the commandline.
type updogSubscriber struct {
	objects []client.Object
	client  client.Client
}

var _ store.SetUpper = &updogSubscriber{}
var _ store.Subscriber = &updogSubscriber{}

func provideUpdogSubscriber(objects []client.Object, client client.Client) *updogSubscriber {
	return &updogSubscriber{
		objects: objects,
		client:  client,
	}
}

func (s *updogSubscriber) SetUp(ctx context.Context, _ store.RStore) error {
	for _, obj := range s.objects {
		// Create() modifies its object mysteriously, so copy it first
		kind := obj.GetObjectKind().GroupVersionKind().Kind
		name := obj.GetName()

		err := s.client.Create(ctx, obj)
		if err != nil {
			return err
		}

		logger.Get(ctx).Infof("loaded %s/%s", strings.ToLower(kind), name)
	}
	return nil
}
func (s *updogSubscriber) OnChange(_ context.Context, _ store.RStore, _ store.ChangeSummary) error {
	return nil
}

func provideUpdogCmdSubscribers(
	hudsc *server.HeadsUpServerController,
	tscm *controllers.TiltServerControllerManager,
	cb *controllers.ControllerBuilder,
	ts *hud.TerminalStream,
	us *updogSubscriber) []store.Subscriber {
	return append(engine.ProvideSubscribersAPIOnly(hudsc, tscm, cb, ts), us)
}

func convertToClientObjects(objs []runtime.Object) ([]ctrlclient.Object, error) {
	result := []ctrlclient.Object{}
	for _, obj := range objs {
		clientObj, ok := obj.(ctrlclient.Object)
		if !ok {
			return nil, fmt.Errorf("Unexpected object type: %T", obj)
		}
		result = append(result, clientObj)
	}
	return result, nil
}
