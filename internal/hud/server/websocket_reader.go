package server

// A thinger to connect to the HUD server websocket, for CLI commands that
// want to read state from a running Tilt (e.g. `tilt logs`).
// TODO(maia): figure out if this should live here, or elsewhere.

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/mattn/go-colorable"
	"github.com/tilt-dev/tilt/internal/hud"
	"github.com/tilt-dev/tilt/internal/hud/webview"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

// TODO: interface
type WebsocketReader struct {
	url     url.URL
	handler ViewHandler
}

func ProvideWebsockerReader() *WebsocketReader {
	return &WebsocketReader{
		url:     url.URL{Scheme: "ws", Host: "localhost:10350", Path: "/ws/view"},
		handler: NewLogStreamer(),
	}
}

type ViewHandler interface {
	Handle(v proto_webview.View) error
}

type LogStreamer struct {
	logstore   *logstore.LogStore
	printer    *hud.IncrementalPrinter
	checkpoint logstore.Checkpoint
}

func NewLogStreamer() *LogStreamer {
	// TODO: wire this
	printer := hud.NewIncrementalPrinter(hud.Stdout(colorable.NewColorableStdout()))
	return &LogStreamer{
		logstore: logstore.NewLogStore(),
		printer:  printer,
	}
}
func (ls *LogStreamer) Handle(v proto_webview.View) error {
	fmt.Printf("✨ got %d log segments\n", len(v.LogList.Segments))
	fromCheckpoint := logstore.Checkpoint(v.LogList.FromCheckpoint)
	toCheckpoint := logstore.Checkpoint(v.LogList.ToCheckpoint)

	fmt.Printf("✨ checkpoints:\n\tfrom: %d\n\tto: %d\n\tls.checkpoint: %d\n",
		fromCheckpoint, toCheckpoint, ls.checkpoint)

	segments := v.LogList.Segments
	if fromCheckpoint < ls.checkpoint {
		// The server is re-sending some logs we already have, so slice them off.
		deleteCount := ls.checkpoint - fromCheckpoint
		segments = segments[deleteCount:]
		fmt.Printf("✨ server resent %d segments\n", deleteCount)
	}

	fmt.Printf("✨ after processing, %d log segments\n", len(segments))
	// TODO(maia): filter for the resources that we care about (`tilt logs resourceA resourceC`)
	//   --> and if there's only one resource, don't prefix logs with resource name?
	for _, seg := range segments {
		// TODO(maia): secrets???
		ls.logstore.Append(webview.LogSegmentToEvent(seg, v.LogList.Spans), model.SecretSet{})
	}

	ls.printer.Print(ls.logstore.ContinuingLines(ls.checkpoint))

	if toCheckpoint > ls.checkpoint {
		ls.checkpoint = toCheckpoint
	}

	return nil
}

func (wsr *WebsocketReader) Listen() {
	// catch signals to kill
	// TODO: use signal catching we already use in `up` etc.
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	log.Printf("connecting to %s", wsr.url.String())

	c, _, err := websocket.DefaultDialer.Dial(wsr.url.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		// TODO(maia): make sure this closes okay 😅
		defer close(done)
		for {
			_, data, err := c.ReadMessage()
			if err != nil {
				log.Println("🚨 error reading:", err)
				return
			}
			// todo: ack for incremental logs
			v := proto_webview.View{}
			jspb := &runtime.JSONPb{OrigName: false, EmitDefaults: true}
			err = jspb.Unmarshal(data, &v)
			if err != nil {
				log.Println("🚨 error unmarshalling:", err)
			}
			err = wsr.handler.Handle(v)
			if err != nil {
				log.Println("🚨 handler error:", err)
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}
