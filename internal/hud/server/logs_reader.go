package server

import "net/url"

// This file defines machinery to connect to the HUD server websocket and
// read logs from a running Tilt instance.
// In future, we can use WebsocketReader more generically to read state
// from a running Tilt, and do different things with that state depending
// on the handler provided (if we ever implement e.g. `tilt status`).
// (If we never use the WebsocketReader elsewhere, we might want to collapse
// it and the LogStreamer handler into a single struct.)

import (
	"bytes"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/gorilla/websocket"
	"github.com/mattn/go-colorable"

	"github.com/tilt-dev/tilt/internal/hud"
	"github.com/tilt-dev/tilt/internal/hud/webview"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
)

// TODO: interface
type WebsocketReader struct {
	url     url.URL
	handler ViewHandler
}

func ProvideWebsockerReader() *WebsocketReader {
	return &WebsocketReader{
		// TODO(maia): pass this URL instead of hardcoding / wire this
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
	// TODO(maia): wire this
	printer := hud.NewIncrementalPrinter(hud.Stdout(colorable.NewColorableStdout()))
	return &LogStreamer{
		logstore: logstore.NewLogStore(),
		printer:  printer,
	}
}
func (ls *LogStreamer) Handle(v proto_webview.View) error {
	fromCheckpoint := logstore.Checkpoint(v.LogList.FromCheckpoint)
	toCheckpoint := logstore.Checkpoint(v.LogList.ToCheckpoint)

	if fromCheckpoint == -1 {
		// Server has no new logs to send
		return nil
	}

	segments := v.LogList.Segments
	if fromCheckpoint < ls.checkpoint {
		// The server is re-sending some logs we already have, so slice them off.
		deleteCount := ls.checkpoint - fromCheckpoint
		segments = segments[deleteCount:]
	}

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

			v := proto_webview.View{}
			unmarshaller := jsonpb.Unmarshaler{}
			err = unmarshaller.Unmarshal(bytes.NewReader(data), &v)
			if err != nil {
				log.Println("🚨 error unmarshalling:", err)
			}
			err = wsr.handler.Handle(v)
			if err != nil {
				log.Println("🚨 handler error:", err)
			}

			toCheckpoint := v.LogList.ToCheckpoint
			if toCheckpoint > 0 {
				// If server is using the incremental logs protocol, ack the
				// message so the next time the websocket sends data, it only
				// sends logs from here on forward
				resp := proto_webview.AckWebsocketRequest{
					ToCheckpoint:  toCheckpoint,
					TiltStartTime: v.TiltStartTime,
				}
				marshaller := jsonpb.Marshaler{OrigName: false, EmitDefaults: true}
				respJson, err := marshaller.MarshalToString(&resp)
				if err != nil {
					log.Println("🚨 marshalling response:", err)
				}

				err = c.WriteMessage(websocket.TextMessage, []byte(respJson))
				if err != nil {
					log.Println("🚨 sending response:", err)
				}
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
