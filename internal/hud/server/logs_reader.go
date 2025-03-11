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
	unmarshaller jsonpb.Unmarshaler
	persistent   bool // whether to keep listening on websocket, or close after first message
	handler      ViewHandler
}

func newWebsocketReaderForLogs(conn WebsocketConn, persistent bool, filter hud.LogFilter, p *hud.IncrementalPrinter) *WebsocketReader {
	ls := NewLogStreamer(filter, p)
	return newWebsocketReader(conn, persistent, ls)
}

func newWebsocketReader(conn WebsocketConn, persistent bool, handler ViewHandler) *WebsocketReader {
	return &WebsocketReader{
		conn:         conn,
		unmarshaller: jsonpb.Unmarshaler{},
		persistent:   persistent,
		handler:      handler,
	}
}

type ViewHandler interface {
	Handle(v *proto_webview.View) error
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
	filter          hud.LogFilter
	printer         *hud.IncrementalPrinter
}

func NewLogStreamer(filter hud.LogFilter, p *hud.IncrementalPrinter) *LogStreamer {
	return &LogStreamer{
		filter:   filter,
		logstore: logstore.NewLogStore(),
		printer:  p,
	}
}

func (ls *LogStreamer) Handle(v *proto_webview.View) error {
	if v == nil || v.LogList == nil || v.LogList.FromCheckpoint == -1 {
		// Server has no new logs to send
		return nil
	}

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

	lines := ls.logstore.ContinuingLinesWithOptions(ls.checkpoint, logstore.LineOptions{
		SuppressPrefix: ls.filter.SuppressPrefix(),
	})
	lines = ls.filter.Apply(lines)
	ls.printer.Print(lines)

	ls.checkpoint = ls.logstore.Checkpoint()
	ls.serverWatermark = v.LogList.ToCheckpoint

	return nil
}

func StreamLogs(ctx context.Context, follow bool, url model.WebURL, filter hud.LogFilter, printer *hud.IncrementalPrinter) error {
	url.Scheme = "ws"
	url.Path = "/ws/view"
	logger.Get(ctx).Debugf("connecting to %s", url.String())

	conn, _, err := websocket.DefaultDialer.Dial(url.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "dialing websocket %s", url.String())
	}
	defer conn.Close()

	wsr := newWebsocketReaderForLogs(conn, follow, filter, printer)
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
					logger.Get(ctx).Errorf("Error streaming logs: %v", err)
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

func (wsr *WebsocketReader) handleTextMessage(_ context.Context, reader io.Reader) error {
	v := &proto_webview.View{}
	err := wsr.unmarshaller.Unmarshal(reader, v)
	if err != nil {
		return errors.Wrap(err, "parsing")
	}

	err = wsr.handler.Handle(v)
	if err != nil {
		return errors.Wrap(err, "handling")
	}

	return nil
}
