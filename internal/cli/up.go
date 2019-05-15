package cli

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/fatih/color"
	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"k8s.io/klog"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/output"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"github.com/windmilleng/tilt/internal/tracer"
)

const DefaultWebPort = 10350
const DefaultWebDevPort = 46764

var updateModeFlag string = string(engine.UpdateModeAuto)
var webModeFlag model.WebMode = model.DefaultWebMode
var webPort = 0
var webDevPort = 0
var logActionsFlag bool = false
var sailEnabled bool = false
var sailModeFlag model.SailMode = model.SailModeProd

type upCmd struct {
	watch      bool
	traceTags  string
	hud        bool
	autoDeploy bool
	fileName   string
}

func (c *upCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up [<name>] [<name2>] [...]",
		Short: "stand up one or more manifests",
	}

	cmd.Flags().BoolVar(&c.watch, "watch", true, "If true, services will be automatically rebuilt and redeployed when files change. Otherwise, each service will be started once.")
	cmd.Flags().Var(&webModeFlag, "web-mode", "Values: local, prod. Controls whether to use prod assets or a local dev server")
	cmd.Flags().StringVar(&updateModeFlag, "update-mode", string(engine.UpdateModeAuto),
		fmt.Sprintf("Control the strategy Tilt uses for updating instances. Possible values: %v", engine.AllUpdateModes))
	cmd.Flags().StringVar(&c.traceTags, "traceTags", "", "tags to add to spans for easy querying, of the form: key1=val1,key2=val2")
	cmd.Flags().StringVar(&build.ImageTagPrefix, "image-tag-prefix", build.ImageTagPrefix,
		"For integration tests. Customize the image tag prefix so tests can write to a public registry")
	cmd.Flags().BoolVar(&c.hud, "hud", true, "If true, tilt will open in HUD mode.")
	cmd.Flags().BoolVar(&c.autoDeploy, "auto-deploy", true, "If false, tilt will wait on <spacebar> to trigger builds")
	cmd.Flags().BoolVar(&logActionsFlag, "logactions", false, "log all actions and state changes")
	cmd.Flags().IntVar(&webPort, "port", DefaultWebPort, "Port for the Tilt HTTP server. Set to 0 to disable.")
	cmd.Flags().IntVar(&webDevPort, "webdev-port", DefaultWebDevPort, "Port for the Tilt Dev Webpack server. Only applies when using --web-mode=local")
	cmd.Flags().BoolVar(&sailEnabled, "share", false, "Enable sharing current state to a remote server")
	cmd.Flags().Var(&sailModeFlag, "share-mode", "Sets the server that we're sharing to. Values: none, default, local, prod, staging")
	cmd.Flags().Lookup("logactions").Hidden = true
	cmd.Flags().StringVar(&c.fileName, "file", tiltfile.FileName, "Path to Tiltfile")
	err := cmd.Flags().MarkHidden("image-tag-prefix")
	if err != nil {
		panic(err)
	}
	err = cmd.Flags().MarkHidden("share-mode")
	if err != nil {
		panic(err)
	}

	return cmd
}

func (c *upCmd) run(ctx context.Context, args []string) error {
	analyticsService.Incr("cmd.up", map[string]string{
		"watch": fmt.Sprintf("%v", c.watch),
		"mode":  string(updateModeFlag),
	})
	defer analyticsService.Flush(time.Second)

	span, ctx := opentracing.StartSpanFromContext(ctx, "Up")
	defer span.Finish()

	tags := tracer.TagStrToMap(c.traceTags)

	for k, v := range tags {
		span.SetTag(k, v)
	}

	threads, err := wireThreads(ctx)
	if err != nil {
		return err
	}

	upper := threads.upper
	h := threads.hud

	l := engine.NewLogActionLogger(ctx, upper.Dispatch)
	ctx = logger.WithLogger(ctx, l)

	log.SetOutput(l.Writer(logger.InfoLvl))
	klog.SetOutput(l.Writer(logger.InfoLvl))

	logOutput(fmt.Sprintf("Starting Tilt (%s)…", buildStamp()))

	if isAnalyticsDisabledFromEnv() {
		logOutput("Tilt analytics manually disabled by environment")
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

	if c.hud {
		err := output.CaptureAllOutput(logger.Get(ctx).Writer(logger.InfoLvl))
		if err != nil {
			logger.Get(ctx).Infof("Error capturing stdout and stderr: %v", err)
		}
		g.Go(func() error {
			err := h.Run(ctx, upper.Dispatch, hud.DefaultRefreshInterval)
			return err
		})
	}

	triggerMode := model.TriggerAuto
	if !c.autoDeploy {
		triggerMode = model.TriggerManual
	}

	g.Go(func() error {
		defer cancel()
		return upper.Start(ctx, args, threads.tiltBuild, c.watch, triggerMode, c.fileName, c.hud, threads.sailMode)
	})

	err = g.Wait()
	if err != context.Canceled {
		return err
	} else {
		return nil
	}
}

func logOutput(s string) {
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
	log.Print(color.GreenString(s))
}

func provideUpdateModeFlag() engine.UpdateModeFlag {
	return engine.UpdateModeFlag(updateModeFlag)
}

func provideLogActions() store.LogActionsFlag {
	return store.LogActionsFlag(logActionsFlag)
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

func provideSailMode() (model.SailMode, error) {
	if !sailEnabled {
		return model.SailModeDisabled, nil
	}

	switch sailModeFlag {
	case model.SailModeLocal, model.SailModeProd, model.SailModeStaging, model.SailModeDisabled:
		return sailModeFlag, nil
	case model.SailModeDefault:
		// TODO(nick): This might eventually change in dev vs prod, but
		// for now, default to disabled.
		return model.SailModeDisabled, nil
	}
	return "", model.UnrecognizedSailModeError(string(sailModeFlag))
}

func provideWebPort() model.WebPort {
	return model.WebPort(webPort)
}

func provideWebDevPort() model.WebDevPort {
	return model.WebDevPort(webDevPort)
}

func provideWebURL(webPort model.WebPort) (model.WebURL, error) {
	if webPort == 0 {
		return model.WebURL{}, nil
	}

	u, err := url.Parse(fmt.Sprintf("http://localhost:%d/", webPort))
	if err != nil {
		return model.WebURL{}, err
	}
	return model.WebURL(*u), nil
}

func provideSailURL(mode model.SailMode) (model.SailURL, error) {
	urlString := ""
	switch mode {
	case model.SailModeLocal:
		urlString = fmt.Sprintf("//localhost:%d/", model.DefaultSailPort)

	case model.SailModeProd:
		urlString = "//sail.tilt.dev/"
	case model.SailModeStaging:
		urlString = "//sail-staging.tilt.dev/"
	}

	if urlString == "" {
		return model.SailURL{}, nil
	}

	u, err := url.Parse(urlString)
	if err != nil {
		return model.SailURL{}, err
	}

	// Base SailURL -- use .Http() and .Ws() methods as appropriate to set scheme
	return model.SailURL(*u), nil
}
