package server

import (
	"context"
	"io"
	"net/url"

	"github.com/golang/protobuf/jsonpb"
	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/pkg/logger"
)

// TODO(maia): rename file to logs_reader.go?
// This file defines machinery to connect to the HUD server websocket and
// read logs from a running Tilt instance.
// In future, we can use WebsocketReader more generically to read state
// from a running Tilt, and do different things with that state depending
// on the handler provided (if we ever implement e.g. `tilt status`).
// (If we never use the WebsocketReader elsewhere, we might want to collapse
// it and the LogStreamer handler into a single struct.)

import (
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
	url          url.URL
	conn         WebsocketConn
	marshaller   jsonpb.Marshaler
	unmarshaller jsonpb.Unmarshaler
	handler      ViewHandler
}

func ProvideWebsockerReader() *WebsocketReader {
	return &WebsocketReader{
		// TODO(maia): pass this URL instead of hardcoding / wire this
		url:          url.URL{Scheme: "ws", Host: "localhost:10350", Path: "/ws/view"},
		handler:      NewLogStreamer(),
		marshaller:   jsonpb.Marshaler{OrigName: false, EmitDefaults: true},
		unmarshaller: jsonpb.Unmarshaler{},
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
	// TODO(maia): wire this (/ maybe this isn't the thing that needs to be wired, but
	//   should be created after we have a conn to pass it?)
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

func (wsr *WebsocketReader) Listen(ctx context.Context) error {
	logger.Get(ctx).Debugf("connecting to %s", wsr.url.String())

	var err error
	wsr.conn, _, err = websocket.DefaultDialer.Dial(wsr.url.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "dialing websocket %s", wsr.url.String())
	}
	defer wsr.conn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			messageType, reader, err := wsr.conn.NextReader()
			if err != nil {
				// uh do i need to do anything with this error? or does it just mean that the socket has closed?
				return
			}

			if messageType == websocket.TextMessage {
				err = wsr.handleTextMessage(ctx, reader)
				if err != nil {
					// will I want this to be an Info sometimes??
					logger.Get(ctx).Verbosef("Error handling websocket message: %v", err)
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

	// If server is using the incremental logs protocol, send back an ACK
	if v.LogList.ToCheckpoint > 0 {
		err = wsr.sendIncrementalLogResp(ctx, &v)
		if err != nil {
			return errors.Wrap(err, "sending websocket ack")
		}
	}
	return nil
}

// Ack a websocket message so the next time the websocket sends data, it only
// sends logs from here on forward
func (wsr *WebsocketReader) sendIncrementalLogResp(ctx context.Context, v *proto_webview.View) error {
	resp := proto_webview.AckWebsocketRequest{
		ToCheckpoint:  v.LogList.ToCheckpoint,
		TiltStartTime: v.TiltStartTime,
	}

	w, err := wsr.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return errors.Wrap(err, "getting writer")
	}
	defer func() {
		err := w.Close()
		if err != nil {
			logger.Get(ctx).Verbosef("closing writer: %v", err)
		}
	}()

	err = wsr.marshaller.Marshal(w, &resp)
	if err != nil {
		return errors.Wrap(err, "sending response")
	}
	return nil
}
