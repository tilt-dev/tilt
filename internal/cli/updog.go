package cli

import (
	"context"
	"log"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/controllers"
	"github.com/tilt-dev/tilt/internal/engine"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/internal/hud"
	"github.com/tilt-dev/tilt/internal/hud/prompt"
	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type updogCmd struct {
}

var _ tiltCmd = &updogCmd{}

func newUpdogCmd() *updogCmd {
	return &updogCmd{}
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
a set of example configs. Here's a youtube video showing it in action:

https://www.youtube.com/watch?v=dQw4w9WgXcQ
`,
		Example: "tilt alpha updog -f config.yaml",
	}

	addStartServerFlags(cmd)

	return cmd
}

func (c *updogCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)

	cmdTags := engineanalytics.CmdTags(map[string]string{})
	a.Incr("cmd.updog", cmdTags.AsMap())
	defer a.Flush(time.Second)

	deferred := logger.NewDeferredLogger(ctx)
	ctx = redirectLogs(ctx, deferred)

	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

	// Force web-mode to prod
	webModeFlag = model.ProdWebMode

	webHost := provideWebHost()
	webURL, _ := provideWebURL(webHost, provideWebPort())
	startLine := prompt.StartStatusLine(webURL, webHost)
	log.Print(startLine)
	log.Print(buildStamp())

	if ok, reason := analytics.IsAnalyticsDisabledFromEnv(); ok {
		log.Printf("Tilt analytics disabled: %s", reason)
	}

	// TODO(nick): Parse objects from the command line rather than
	// hard-coding them here. See 'ctlptl apply' for example code.
	objects := []client.Object{
		&v1alpha1.Cmd{
			ObjectMeta: metav1.ObjectMeta{
				Name: "hello-world",
			},
			Spec: v1alpha1.CmdSpec{
				Args: []string{"echo", "hello world"},
			},
		},
	}

	deps, err := wireCmdUpdog(ctx, a, nil, "updog", objects)
	if err != nil {
		deferred.SetOutput(deferred.Original())
		return err
	}

	l := store.NewLogActionLogger(ctx, deps.Upper.Dispatch)
	deferred.SetOutput(l)
	ctx = redirectLogs(ctx, l)

	// A lot of these parameters don't matter because we don't have any
	// controllers registered.
	err = deps.Upper.Start(ctx, args, deps.TiltBuild, store.EngineModeCI,
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
		err := s.client.Create(ctx, obj)
		if err != nil {
			return err
		}
	}
	return nil
}
func (s *updogSubscriber) OnChange(_ context.Context, _ store.RStore, _ store.ChangeSummary) {
}

func provideUpdogCmdSubscribers(
	hudsc *server.HeadsUpServerController,
	tscm *controllers.TiltServerControllerManager,
	cb *controllers.ControllerBuilder,
	ts *hud.TerminalStream,
	us *updogSubscriber) []store.Subscriber {
	return append(engine.ProvideSubscribersAPIOnly(hudsc, tscm, cb, ts), us)
}
