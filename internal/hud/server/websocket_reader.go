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
	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

type ViewHandler func(v proto_webview.View) error

// TODO: interface
type WebsocketReader struct {
	url     url.URL
	handler ViewHandler
}

var rName = "make-install"

func StreamLogs(v proto_webview.View) error {
	fmt.Println("found resources:", v.Resources)
	return nil
}

func ProvideWebsockerReader() *WebsocketReader {
	return &WebsocketReader{
		url:     url.URL{Scheme: "ws", Host: "localhost:10350", Path: "/ws/view"},
		handler: StreamLogs,
	}
}

func (wsr *WebsocketReader) Listen() {
	// catch signals to kill
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
		defer close(done)
		for {
			_, data, err := c.ReadMessage()
			if err != nil {
				log.Println("🚨 error reading:", err)
				return
			}
			v := proto_webview.View{}
			jspb := &runtime.JSONPb{OrigName: false, EmitDefaults: true}
			err = jspb.Unmarshal(data, &v)
			if err != nil {
				log.Println("🚨 error unmarshalling:", err)
			}
			err = wsr.handler(v)
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
