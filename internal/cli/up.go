package cli

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/controllers"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/internal/hud/prompt"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/web"
)

var webModeFlag model.WebMode = model.DefaultWebMode

const DefaultWebDevPort = 46764

var (
	updateModeFlag   string   = string(liveupdates.UpdateModeAuto)
	webDevPort                = 0
	logActionsFlag   bool     = false
	logSourceFlag    string   = ""
	logResourcesFlag []string = nil
	logLevelFlag     string   = ""
)

var userExitError = errors.New("user requested Tilt exit")

//go:embed Tiltfile.starter
var starterTiltfile []byte

type upCmd struct {
	fileName             string
	outputSnapshotOnExit string

	legacy bool
	stream bool
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
In that case, see https://docs.tilt.dev/tiltfile_config.html and/or comments in your Tiltfile

When you exit Tilt (using Ctrl+C), Kubernetes resources and Docker Compose resources continue running;
you can use tilt down (https://docs.tilt.dev/cli/tilt_down.html) to delete these resources. Any long-running
local resources--i.e. those using serve_cmd--are terminated when you exit Tilt.
`,
	}

	cmd.Flags().StringVar(&updateModeFlag, "update-mode", string(liveupdates.UpdateModeAuto),
		fmt.Sprintf("Control the strategy Tilt uses for updating instances. Possible values: %v", liveupdates.AllUpdateModes))
	cmd.Flags().BoolVar(&c.legacy, "legacy", false, "If true, tilt will open in legacy terminal mode.")
	cmd.Flags().BoolVar(&c.stream, "stream", false, "If true, tilt will stream logs in the terminal.")
	cmd.Flags().BoolVar(&logActionsFlag, "logactions", false, "log all actions and state changes")
	addStartServerFlags(cmd)
	addDevServerFlags(cmd)
	addTiltfileFlag(cmd, &c.fileName)
	addKubeContextFlag(cmd)
	addNamespaceFlag(cmd)
	addLogFilterFlags(cmd, "log-")
	addLogFilterResourcesFlag(cmd)
	cmd.Flags().Lookup("logactions").Hidden = true
	cmd.Flags().StringVar(&c.outputSnapshotOnExit, "output-snapshot-on-exit", "", "If specified, Tilt will dump a snapshot of its state to the specified path when it exits")

	return cmd
}

func (c *upCmd) initialTermMode(isTerminal bool) store.TerminalMode {
	if !isTerminal {
		return store.TerminalModeStream
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	a := analytics.Get(ctx)
	defer a.Flush(time.Second)

	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
	isTTY := isatty.IsTerminal(os.Stdout.Fd())
	termMode := c.initialTermMode(isTTY)

	cmdUpTags := engineanalytics.CmdTags(map[string]string{
		"update_mode": updateModeFlag, // before 7/8/20 this was just called "mode"
		"term_mode":   strconv.Itoa(int(termMode)),
	})

	generateTiltfileResult, err := maybeGenerateTiltfile(c.fileName)
	// N.B. report the command before handling the error; result enum is always valid
	cmdUpTags["generate_tiltfile.result"] = string(generateTiltfileResult)
	a.Incr("cmd.up", cmdUpTags.AsMap())
	if err == userExitError {
		return nil
	} else if err != nil {
		return err
	}

	deferred := logger.NewDeferredLogger(ctx)
	ctx = redirectLogs(ctx, deferred)

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
	controllers.MaybeSetKlogOutput(l.Writer(logger.InfoLvl))
	return ctx
}

func provideUpdateModeFlag() liveupdates.UpdateModeFlag {
	return liveupdates.UpdateModeFlag(updateModeFlag)
}

func provideLogActions() store.LogActionsFlag {
	return store.LogActionsFlag(logActionsFlag)
}

func provideWebMode(b model.TiltBuild) (model.WebMode, error) {
	switch webModeFlag {
	case model.LocalWebMode,
		model.ProdWebMode,
		model.EmbeddedWebMode,
		model.PrecompiledWebMode:
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

func targetMode(mode model.WebMode, embeddedAvailable bool) (model.WebMode, error) {
	if (mode == model.EmbeddedWebMode || mode == model.PrecompiledWebMode) && !embeddedAvailable {
		return mode, fmt.Errorf(
			("requested %s mode, but JS/CSS files are not available.\n" +
				"Please report this: https://github.com/tilt-dev/tilt/issues"), string(mode))
	}
	if mode.IsProd() {
		// defaults to embedded, reporting an error if embedded not available.
		if !embeddedAvailable {
			return mode, fmt.Errorf(
				("running in prod mode, but JS/CSS files are not available.\n" +
					"Please report this: https://github.com/tilt-dev/tilt/issues"))
		} else if mode == model.ProdWebMode {
			mode = model.EmbeddedWebMode
		}
	} else { // precompiled when available and by request, otherwise local
		if mode != model.PrecompiledWebMode {
			mode = model.LocalWebMode
		}
	}
	return mode, nil
}

func provideAssetServer(mode model.WebMode, version model.WebVersion) (assets.Server, error) {
	s, ok := assets.GetEmbeddedServer()
	m, err := targetMode(mode, ok)
	if err != nil {
		return nil, err
	}

	switch m {
	case model.EmbeddedWebMode, model.PrecompiledWebMode:
		return s, nil
	case model.LocalWebMode:
		path, err := web.StaticPath()
		if err != nil {
			return nil, err
		}
		pkgDir := assets.PackageDir(path)
		return assets.NewDevServer(pkgDir, model.WebDevPort(webDevPort))
	}
	return nil, model.UnrecognizedWebModeError(string(mode))
}
