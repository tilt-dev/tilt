package sail

import (
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/cobra"
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
	ss := server.ProvideSailServer()
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.DefaultServeMux,
	}
	http.Handle("/", ss.Router())

	log.Printf("Sail server listening on %d\n", port)
	err := httpServer.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
