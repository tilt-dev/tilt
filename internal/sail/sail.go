package sail

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	hudServer "github.com/windmilleng/tilt/internal/hud/server"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/network"
	"github.com/windmilleng/tilt/internal/sail/server"
	"golang.org/x/sync/errgroup"
)

const DefaultWebDevPort = 10451

var webModeFlag model.WebMode = model.DefaultWebMode
var webDevPort = DefaultWebDevPort
var port = model.DefaultSailPort

func Execute() {
	rootCmd := &cobra.Command{
		Use:   "sail",
		Short: "A server to coordinate collaborative coding and debugging with Tilt",
		Run:   run,
	}
	rootCmd.Flags().IntVar(&port, "port", model.DefaultSailPort, "Port to listen on")
	rootCmd.Flags().Var(&webModeFlag, "web-mode", "Values: local, prod. Controls whether to use prod assets or a local dev server")
	rootCmd.Flags().IntVar(&webDevPort, "webdev-port", DefaultWebDevPort, "Port for the Tilt Dev Webpack server. Only applies when using --web-mode=local")

	err := rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}

func contextWithCancel() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	l := logger.NewLogger(logger.InfoLvl, os.Stderr)
	ctx = logger.WithLogger(ctx, l)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs

		cancel()

		// If we get another SIGINT/SIGTERM, OR it takes too long for tilt to
		// exit after cancelling context, just exit
		select {
		case <-sigs:
			l.Debugf("force quitting...")
			os.Exit(1)
		case <-time.After(2 * time.Second):
			l.Debugf("Context canceled but app still running; forcibly exiting.")
			os.Exit(1)
		}
	}()

	return ctx, cancel
}

func run(cmd *cobra.Command, args []string) {
	ctx, cancel := contextWithCancel()

	mode, err := provideWebMode()
	if err != nil {
		log.Fatal(err)
	}

	assets, err := hudServer.ProvideAssetServer(ctx, mode, model.WebVersion("0.0.0"), provideWebDevPort())
	if err != nil {
		log.Fatal(err)
	}

	ss := server.ProvideSailServer(assets)
	httpServer := &http.Server{
		Addr:    network.AllHostsBindAddr(int(port)),
		Handler: http.DefaultServeMux,
	}
	http.Handle("/", ss.Router())

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		defer cancel()

		log.Printf("Sail server listening on %d\n", port)
		return httpServer.ListenAndServe()
	})

	g.Go(func() error {
		defer cancel()
		return assets.Serve(ctx)
	})

	g.Go(func() error {
		<-ctx.Done()
		_ = httpServer.Shutdown(context.Background())
		assets.Teardown(context.Background())
		return nil
	})

	err = g.Wait()
	if err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}

func provideWebMode() (model.WebMode, error) {
	switch webModeFlag {
	case model.LocalWebMode, model.ProdWebMode, model.PrecompiledWebMode:
		return webModeFlag, nil
	case model.DefaultWebMode:
		return model.LocalWebMode, nil
	}
	return "", model.UnrecognizedWebModeError(string(webModeFlag))
}

func provideWebDevPort() model.WebDevPort {
	return model.WebDevPort(webDevPort)
}
