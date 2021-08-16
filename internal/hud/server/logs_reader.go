package server

import (
	"context"
	"io"

	"github.com/golang/protobuf/jsonpb"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/hud"
	"github.com/tilt-dev/tilt/internal/hud/webview"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
)

// This file defines machinery to connect to the HUD server websocket and
// read logs from a running Tilt instance.
// In future, we can use WebsocketReader more generically to read state
// from a running Tilt, and do different things with that state depending
// on the handler provided (if we ever implement e.g. `tilt status`).
// (If we never use the WebsocketReader elsewhere, we might want to collapse
// it and the LogStreamer handler into a single struct.)

type WebsocketReader struct {
	conn         WebsocketConn
	marshaller   jsonpb.Marshaler
	unmarshaller jsonpb.Unmarshaler
	persistent   bool // whether to keep listening on websocket, or close after first message
	handler      ViewHandler
}

func newWebsocketReaderForLogs(conn WebsocketConn, persistent bool, resources []string, p *hud.IncrementalPrinter) *WebsocketReader {
	ls := NewLogStreamer(resources, p)
	return newWebsocketReader(conn, persistent, ls)
}

func newWebsocketReader(conn WebsocketConn, persistent bool, handler ViewHandler) *WebsocketReader {
	return &WebsocketReader{
		conn:         conn,
		marshaller:   jsonpb.Marshaler{},
		unmarshaller: jsonpb.Unmarshaler{},
		persistent:   persistent,
		handler:      handler,
	}
}

type ViewHandler interface {
	Handle(v proto_webview.View) error
}

type LogStreamer struct {
	logstore *logstore.LogStore
	// checkpoint tracks the client's latest printed logs.
	//
	// WARNING: The server watermark values CANNOT be used for checkpointing within the client!
	checkpoint logstore.Checkpoint
	// serverWatermark ensures that we don't print any duplicate logs.
	//
	// This value should only be used to compare to other server values, NOT client checkpoints.
	serverWatermark int32
	resources       model.ManifestNameSet // if present, resource(s) to stream logs for
	printer         *hud.IncrementalPrinter
}

func NewLogStreamer(resources []string, p *hud.IncrementalPrinter) *LogStreamer {
	mnSet := make(map[model.ManifestName]bool, len(resources))
	for _, r := range resources {
		mnSet[model.ManifestName(r)] = true
	}

	return &LogStreamer{
		resources: mnSet,
		logstore:  logstore.NewLogStore(),
		printer:   p,
	}
}

func (ls *LogStreamer) Handle(v proto_webview.View) error {
	if v.LogList == nil || v.LogList.FromCheckpoint == -1 {
		// Server has no new logs to send
		return nil
	}

	// if printing logs for only one resource, don't need resource name prefix
	suppressPrefix := len(ls.resources) == 1

	segments := v.LogList.Segments
	if v.LogList.FromCheckpoint < ls.serverWatermark {
		// The server is re-sending some logs we already have, so slice them off.
		deleteCount := ls.serverWatermark - v.LogList.FromCheckpoint
		segments = segments[deleteCount:]
	}

	for _, seg := range segments {
		// TODO(maia): secrets???
		ls.logstore.Append(webview.LogSegmentToEvent(seg, v.LogList.Spans), model.SecretSet{})
	}

	ls.printer.Print(ls.logstore.ContinuingLinesWithOptions(ls.checkpoint, logstore.LineOptions{
		ManifestNames:  ls.resources,
		SuppressPrefix: suppressPrefix,
	}))

	ls.checkpoint = ls.logstore.Checkpoint()
	ls.serverWatermark = v.LogList.ToCheckpoint

	return nil
}
func StreamLogs(ctx context.Context, follow bool, url model.WebURL, resources []string, printer *hud.IncrementalPrinter) error {
	url.Scheme = "ws"
	url.Path = "/ws/view"
	logger.Get(ctx).Debugf("connecting to %s", url.String())

	conn, _, err := websocket.DefaultDialer.Dial(url.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "dialing websocket %s", url.String())
	}
	defer conn.Close()

	wsr := newWebsocketReaderForLogs(conn, follow, resources, printer)
	return wsr.Listen(ctx)
}

func (wsr *WebsocketReader) Listen(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			messageType, reader, err := wsr.conn.NextReader()
			if err != nil {
				return
			}

			if messageType == websocket.TextMessage {
				err = wsr.handleTextMessage(ctx, reader)
				if err != nil {
					// will I want this to be an Info sometimes??
					logger.Get(ctx).Verbosef("Error handling websocket message: %v", err)
				}
				if !wsr.persistent {
					return
				}
			}
		}
	}()

	for {
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			err := ctx.Err()
			if err != context.Canceled {
				return err
			}

			return wsr.conn.Close()
		}
	}
}

func (wsr *WebsocketReader) handleTextMessage(ctx context.Context, reader io.Reader) error {
	v := proto_webview.View{}
	err := wsr.unmarshaller.Unmarshal(reader, &v)
	if err != nil {
		return errors.Wrap(err, "Unmarshalling websocket message")
	}

	err = wsr.handler.Handle(v)
	if err != nil {
		return errors.Wrap(err, "Handling Tilt state from websocket")
	}

	return nil
}
