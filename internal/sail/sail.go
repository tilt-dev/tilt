package sail

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	hudServer "github.com/windmilleng/tilt/internal/hud/server"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/sail/server"
)

var port = 0

func Execute() {
	rootCmd := &cobra.Command{
		Use:   "sail",
		Short: "A server to coordinate collaborative coding and debugging with Tilt",
		Run:   run,
	}
	rootCmd.Flags().IntVar(&port, "port", model.DefaultSailPort, "Port to listen on")

	err := rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = logger.WithLogger(ctx, logger.NewLogger(logger.InfoLvl, os.Stderr))

	// TODO(nick): This is a quick hack to get something working,
	// and will eventually need more controls.
	assets, err := hudServer.ProvideAssetServer(ctx, model.LocalWebMode, model.WebVersion("0.0.0"), model.WebDevPort(10451))
	if err != nil {
		log.Fatal(err)
	}

	ss := server.ProvideSailServer(assets)
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
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

	err = g.Wait()
	if err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}
