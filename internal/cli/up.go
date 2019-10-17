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

	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/web"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/output"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"github.com/windmilleng/tilt/internal/tracer"
	"github.com/windmilleng/tilt/pkg/assets"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

const DefaultWebPort = 10350
const DefaultWebDevPort = 46764

var updateModeFlag string = string(engine.UpdateModeAuto)
var webModeFlag model.WebMode = model.DefaultWebMode
var webPort = 0
var webDevPort = 0
var noBrowser bool = false
var logActionsFlag bool = false

type upCmd struct {
	watch     bool
	traceTags string
	hud       bool
	fileName  string
}

func (c *upCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up [<resource_name1>] [<resource_name2>] [...]",
		Short: "stand up one or more resources (if no resource names specified, stand up all known resources specified in Tiltfile)",
	}

	cmd.Flags().BoolVar(&c.watch, "watch", true, "If true, services will be automatically rebuilt and redeployed when files change. Otherwise, each service will be started once.")
	cmd.Flags().Var(&webModeFlag, "web-mode", "Values: local, prod. Controls whether to use prod assets or a local dev server")
	cmd.Flags().StringVar(&updateModeFlag, "update-mode", string(engine.UpdateModeAuto),
		fmt.Sprintf("Control the strategy Tilt uses for updating instances. Possible values: %v", engine.AllUpdateModes))
	cmd.Flags().StringVar(&c.traceTags, "traceTags", "", "tags to add to spans for easy querying, of the form: key1=val1,key2=val2")
	cmd.Flags().BoolVar(&c.hud, "hud", true, "If true, tilt will open in HUD mode.")
	cmd.Flags().BoolVar(&logActionsFlag, "logactions", false, "log all actions and state changes")
	cmd.Flags().IntVar(&webPort, "port", DefaultWebPort, "Port for the Tilt HTTP server. Set to 0 to disable.")
	cmd.Flags().IntVar(&webDevPort, "webdev-port", DefaultWebDevPort, "Port for the Tilt Dev Webpack server. Only applies when using --web-mode=local")
	cmd.Flags().Lookup("logactions").Hidden = true
	cmd.Flags().StringVar(&c.fileName, "file", tiltfile.FileName, "Path to Tiltfile")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "If true, web UI will not open on startup.")

	return cmd
}

func (c *upCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	a.Incr("cmd.up", map[string]string{
		"watch": fmt.Sprintf("%v", c.watch),
		"mode":  string(updateModeFlag),
	})
	a.IncrIfUnopted("analytics.up.optdefault")
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

	if isAnalyticsDisabledFromEnv() {
		logOutput("Tilt analytics manually disabled by environment")
	}

	threads, err := wireThreads(ctx, a)
	if err != nil {
		deferred.SetOutput(deferred.Original())
		return err
	}

	upper := threads.upper
	h := threads.hud

	l := store.NewLogActionLogger(ctx, upper.Dispatch)
	deferred.SetOutput(l)
	ctx = redirectLogs(ctx, l)

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

	g.Go(func() error {
		defer cancel()
		return upper.Start(ctx, args, threads.tiltBuild, c.watch, c.fileName, c.hud, a.Opt(), threads.token, string(threads.cloudAddress))
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

func provideUpdateModeFlag() engine.UpdateModeFlag {
	return engine.UpdateModeFlag(updateModeFlag)
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

func provideWebPort() model.WebPort {
	return model.WebPort(webPort)
}

func provideNoBrowserFlag() model.NoBrowser {
	return model.NoBrowser(noBrowser)
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
