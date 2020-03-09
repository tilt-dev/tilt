package cli

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"k8s.io/klog"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/cloud"
	engineanalytics "github.com/windmilleng/tilt/internal/engine/analytics"
	"github.com/windmilleng/tilt/internal/engine/buildcontrol"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"github.com/windmilleng/tilt/internal/tracer"
	"github.com/windmilleng/tilt/pkg/assets"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/web"
)

const DefaultWebHost = "localhost"
const DefaultWebPort = 10350
const DefaultWebDevPort = 46764

var updateModeFlag string = string(buildcontrol.UpdateModeAuto)
var webModeFlag model.WebMode = model.DefaultWebMode
var webPort = 0
var webHost = DefaultWebHost
var webDevPort = 0
var noBrowser bool = false
var logActionsFlag bool = false

type upCmd struct {
	watch                bool
	traceTags            string
	hud                  bool
	fileName             string
	outputSnapshotOnExit string
}

func (c *upCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "up [<tilt flags>] [-- <Tiltfile args>]",
		DisableFlagsInUseLine: true,
		Short:                 "Start Tilt with the given Tiltfile args",
		Long: `
Starts Tilt and runs services defined in the Tiltfile.

There are two types of args:
1) Tilt flags, listed below, which are handled entirely by Tilt.
2) Tiltfile args, which can be anything, and are potentially accessed by config.parse in your Tiltfile.

By default:
1) Tiltfile args are interpreted as the list of services to start, e.g. tilt up frontend backend.
2) Running with no Tiltfile args starts all services defined in the Tiltfile

This default behavior does not apply if the Tiltfile uses config.parse or config.set_enabled_resources.
In that case, see https://tilt.dev/user_config.html and/or comments in your Tiltfile
`,
	}

	cmd.Flags().BoolVar(&c.watch, "watch", true, "If true, services will be automatically rebuilt and redeployed when files change. Otherwise, each service will be started once.")
	cmd.Flags().Var(&webModeFlag, "web-mode", "Values: local, prod. Controls whether to use prod assets or a local dev server")
	cmd.Flags().StringVar(&updateModeFlag, "update-mode", string(buildcontrol.UpdateModeAuto),
		fmt.Sprintf("Control the strategy Tilt uses for updating instances. Possible values: %v", buildcontrol.AllUpdateModes))
	cmd.Flags().StringVar(&c.traceTags, "traceTags", "", "tags to add to spans for easy querying, of the form: key1=val1,key2=val2")
	cmd.Flags().BoolVar(&c.hud, "hud", true, "If true, tilt will open in HUD mode.")
	cmd.Flags().BoolVar(&logActionsFlag, "logactions", false, "log all actions and state changes")
	cmd.Flags().IntVar(&webPort, "port", DefaultWebPort, "Port for the Tilt HTTP server. Set to 0 to disable.")
	cmd.Flags().StringVar(&webHost, "host", DefaultWebHost, "Host for the Tilt HTTP server and default host for any port-forwards. Set to 0.0.0.0 to listen on all interfaces.")
	cmd.Flags().IntVar(&webDevPort, "webdev-port", DefaultWebDevPort, "Port for the Tilt Dev Webpack server. Only applies when using --web-mode=local")
	cmd.Flags().Lookup("logactions").Hidden = true
	cmd.Flags().StringVar(&c.fileName, "file", tiltfile.FileName, "Path to Tiltfile")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "If true, web UI will not open on startup.")
	cmd.Flags().StringVar(&c.outputSnapshotOnExit, "output-snapshot-on-exit", "", "If specified, Tilt will dump a snapshot of its state to the specified path when it exits")

	return cmd
}

func (c *upCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	cmdUpTags := engineanalytics.CmdUpTags(map[string]string{
		"watch": fmt.Sprintf("%v", c.watch),
		"mode":  string(updateModeFlag),
	})
	a.Incr("cmd.up", cmdUpTags.AsMap())
	defer a.Flush(time.Second)

	span, ctx := opentracing.StartSpanFromContext(ctx, "Up")
	defer span.Finish()

	tags := tracer.TagStrToMap(c.traceTags)

	for k, v := range tags {
		span.SetTag(k, v)
	}

	deferred := logger.NewDeferredLogger(ctx)
	ctx = redirectLogs(ctx, deferred)

	logOutput(fmt.Sprintf("Starting Tilt (%s)…", buildStamp()))

	if ok, reason := analytics.IsAnalyticsDisabledFromEnv(); ok {
		log.Printf("Tilt analytics disabled: %s", reason)
	}

	hudEnabled := c.hud && isatty.IsTerminal(os.Stdout.Fd())
	cmdUpDeps, err := wireCmdUp(ctx, hud.HudEnabled(hudEnabled), a, cmdUpTags)
	if err != nil {
		deferred.SetOutput(deferred.Original())
		return err
	}

	upper := cmdUpDeps.Upper
	h := cmdUpDeps.Hud

	l := store.NewLogActionLogger(ctx, upper.Dispatch)
	deferred.SetOutput(l)
	ctx = redirectLogs(ctx, l)
	if c.outputSnapshotOnExit != "" {
		defer cloud.WriteSnapshot(ctx, cmdUpDeps.Store, c.outputSnapshotOnExit)
	}

	if trace {
		traceID, err := tracer.TraceID(ctx)
		if err != nil {
			return err
		}
		logger.Get(ctx).Infof("TraceID: %s", traceID)
	}

	g, ctx := errgroup.WithContext(ctx)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if hudEnabled {
		g.Go(func() error {
			err := h.Run(ctx, upper.Dispatch, hud.DefaultRefreshInterval)
			return err
		})
	}

	g.Go(func() error {
		defer cancel()
		return upper.Start(ctx, args, cmdUpDeps.TiltBuild, c.watch, c.fileName, hudEnabled, a.UserOpt(), cmdUpDeps.Token, string(cmdUpDeps.CloudAddress))
	})

	err = g.Wait()
	if err != context.Canceled {
		return err
	} else {
		return nil
	}
}

func redirectLogs(ctx context.Context, l logger.Logger) context.Context {
	ctx = logger.WithLogger(ctx, l)
	log.SetOutput(l.Writer(logger.InfoLvl))
	klog.SetOutput(l.Writer(logger.InfoLvl))
	return ctx
}

func logOutput(s string) {
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
	log.Print(color.GreenString(s))
}

func provideUpdateModeFlag() buildcontrol.UpdateModeFlag {
	return buildcontrol.UpdateModeFlag(updateModeFlag)
}

func provideLogActions() store.LogActionsFlag {
	return store.LogActionsFlag(logActionsFlag)
}

func provideKubectlLogLevel() k8s.KubectlLogLevel {
	return k8s.KubectlLogLevel(klogLevel)
}

func provideWebMode(b model.TiltBuild) (model.WebMode, error) {
	switch webModeFlag {
	case model.LocalWebMode, model.ProdWebMode, model.PrecompiledWebMode:
		return webModeFlag, nil
	case model.DefaultWebMode:
		if b.Dev {
			return model.LocalWebMode, nil
		} else {
			return model.ProdWebMode, nil
		}
	}
	return "", model.UnrecognizedWebModeError(string(webModeFlag))
}

func provideWebHost() model.WebHost {
	return model.WebHost(webHost)
}

func provideWebPort() model.WebPort {
	return model.WebPort(webPort)
}

func provideNoBrowserFlag() model.NoBrowser {
	return model.NoBrowser(noBrowser)
}

func provideWebURL(webHost model.WebHost, webPort model.WebPort) (model.WebURL, error) {
	if webPort == 0 {
		return model.WebURL{}, nil
	}

	u, err := url.Parse(fmt.Sprintf("http://%s:%d/", webHost, webPort))
	if err != nil {
		return model.WebURL{}, err
	}
	return model.WebURL(*u), nil
}

func provideAssetServer(mode model.WebMode, version model.WebVersion) (assets.Server, error) {
	if mode == model.ProdWebMode {
		return assets.NewProdServer(assets.ProdAssetBucket, version)
	}
	if mode == model.PrecompiledWebMode || mode == model.LocalWebMode {
		path, err := web.StaticPath()
		if err != nil {
			return nil, err
		}
		pkgDir := assets.PackageDir(path)
		if mode == model.PrecompiledWebMode {
			return assets.NewPrecompiledServer(pkgDir), nil
		} else {
			return assets.NewDevServer(pkgDir, model.WebDevPort(webDevPort))
		}
	}
	return nil, model.UnrecognizedWebModeError(string(mode))
}
