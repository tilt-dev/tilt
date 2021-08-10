package cli

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/hud/prompt"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/web"
)

var webModeFlag model.WebMode = model.DefaultWebMode

const DefaultWebDevPort = 46764

var updateModeFlag string = string(buildcontrol.UpdateModeAuto)
var webDevPort = 0
var logActionsFlag bool = false

type upCmd struct {
	fileName             string
	outputSnapshotOnExit string

	hud    bool
	legacy bool
	stream bool
	// whether hud/legacy/stream flags were explicitly set or just got the default value
	hudFlagExplicitlySet bool
}

func (c *upCmd) name() model.TiltSubcommand { return "up" }

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

When you exit Tilt (using Ctrl+C), Kubernetes resources and Docker Compose resources continue running;
you can use tilt down (https://docs.tilt.dev/cli/tilt_down.html) to delete these resources. Any long-running
local resources--i.e. those using serve_cmd--are terminated when you exit Tilt.
`,
	}

	cmd.Flags().StringVar(&updateModeFlag, "update-mode", string(buildcontrol.UpdateModeAuto),
		fmt.Sprintf("Control the strategy Tilt uses for updating instances. Possible values: %v", buildcontrol.AllUpdateModes))
	cmd.Flags().BoolVar(&c.hud, "hud", true, "If true, tilt will open in HUD mode.")
	cmd.Flags().BoolVar(&c.legacy, "legacy", false, "If true, tilt will open in legacy terminal mode.")
	cmd.Flags().BoolVar(&c.stream, "stream", false, "If true, tilt will stream logs in the terminal.")
	cmd.Flags().BoolVar(&logActionsFlag, "logactions", false, "log all actions and state changes")
	addStartServerFlags(cmd)
	addDevServerFlags(cmd)
	addTiltfileFlag(cmd, &c.fileName)
	addKubeContextFlag(cmd)
	addNamespaceFlag(cmd)
	cmd.Flags().Lookup("logactions").Hidden = true
	cmd.Flags().StringVar(&c.outputSnapshotOnExit, "output-snapshot-on-exit", "", "If specified, Tilt will dump a snapshot of its state to the specified path when it exits")

	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		c.hudFlagExplicitlySet = cmd.Flag("hud").Changed
	}

	return cmd
}

func (c *upCmd) initialTermMode(isTerminal bool) store.TerminalMode {
	if !isTerminal {
		return store.TerminalModeStream
	}

	if c.hudFlagExplicitlySet {
		if c.hud {
			return store.TerminalModeHUD
		}
	}

	if c.legacy {
		return store.TerminalModeHUD
	}

	if c.stream {
		return store.TerminalModeStream
	}

	return store.TerminalModePrompt
}

func (c *upCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)

	termMode := c.initialTermMode(isatty.IsTerminal(os.Stdout.Fd()))

	cmdUpTags := engineanalytics.CmdTags(map[string]string{
		"update_mode": updateModeFlag, // before 7/8/20 this was just called "mode"
		"term_mode":   strconv.Itoa(int(termMode)),
	})
	a.Incr("cmd.up", cmdUpTags.AsMap())
	defer a.Flush(time.Second)

	deferred := logger.NewDeferredLogger(ctx)
	ctx = redirectLogs(ctx, deferred)

	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

	webHost := provideWebHost()
	webURL, _ := provideWebURL(webHost, provideWebPort())
	startLine := prompt.StartStatusLine(webURL, webHost)
	log.Print(startLine)
	log.Print(buildStamp())

	if ok, reason := analytics.IsAnalyticsDisabledFromEnv(); ok {
		log.Printf("Tilt analytics disabled: %s", reason)
	}

	cmdUpDeps, err := wireCmdUp(ctx, a, cmdUpTags, "up")
	if err != nil {
		deferred.SetOutput(deferred.Original())
		return err
	}

	upper := cmdUpDeps.Upper
	if termMode == store.TerminalModePrompt {
		// Any logs that showed up during initialization, make sure they're
		// in the prompt.
		cmdUpDeps.Prompt.SetInitOutput(deferred.CopyBuffered(logger.InfoLvl))
	}

	l := store.NewLogActionLogger(ctx, upper.Dispatch)
	deferred.SetOutput(l)
	ctx = redirectLogs(ctx, l)
	if c.outputSnapshotOnExit != "" {
		defer cmdUpDeps.Snapshotter.WriteSnapshot(ctx, c.outputSnapshotOnExit)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err = upper.Start(ctx, args, cmdUpDeps.TiltBuild,
		c.fileName, termMode, a.UserOpt(), cmdUpDeps.Token, string(cmdUpDeps.CloudAddress))
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

func provideUpdateModeFlag() buildcontrol.UpdateModeFlag {
	return buildcontrol.UpdateModeFlag(updateModeFlag)
}

func provideLogActions() store.LogActionsFlag {
	return store.LogActionsFlag(logActionsFlag)
}

func provideWebMode(b model.TiltBuild) (model.WebMode, error) {
	switch webModeFlag {
	case model.LocalWebMode, model.ProdWebMode, model.PrecompiledWebMode:
		return webModeFlag, nil
	case model.DefaultWebMode:
		// Set prod web mode from an environment variable. Useful for
		// running integration tests against dev tilt.
		webMode := os.Getenv("TILT_WEB_MODE")
		if webMode == "prod" {
			return model.ProdWebMode, nil
		}

		if b.Dev {
			return model.LocalWebMode, nil
		} else {
			return model.ProdWebMode, nil
		}
	}
	return "", model.UnrecognizedWebModeError(string(webModeFlag))
}

func provideWebHost() model.WebHost {
	return model.WebHost(webHostFlag)
}

func provideWebPort() model.WebPort {
	return model.WebPort(webPortFlag)
}

func provideWebURL(webHost model.WebHost, webPort model.WebPort) (model.WebURL, error) {
	if webPort == 0 {
		return model.WebURL{}, nil
	}

	if webHost == "0.0.0.0" {
		// 0.0.0.0 means "listen on all hosts"
		// For UI displays, we use 127.0.0.1 (loopback)
		webHost = "127.0.0.1"
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
