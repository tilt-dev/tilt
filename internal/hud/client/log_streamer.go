package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/hud/webview"
	"github.com/tilt-dev/tilt/internal/token"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
)

type FollowFlag bool

type LogStreamer struct {
	follow  FollowFlag
	url     model.WebURL
	filter  LogFilter
	printer LogPrinter
}

func NewLogStreamer(follow FollowFlag, url model.WebURL, filter LogFilter, printer LogPrinter) *LogStreamer {
	return &LogStreamer{
		follow:  follow,
		url:     url,
		filter:  filter,
		printer: printer,
	}
}

func (ls *LogStreamer) Stream(ctx context.Context) error {
	csrfToken, err := ls.fetchWebsocketToken(ctx)
	if err != nil {
		return errors.Wrap(err, "fetching websocket token")
	}

	wsURL := ls.url
	wsURL.Scheme = "ws"
	wsURL.Path = "/ws/view"
	wsURL.RawQuery = url.Values{"csrf": []string{csrfToken}}.Encode()
	logger.Get(ctx).Debugf("connecting to %s", wsURL.String())

	conn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "dialing websocket %s", wsURL.String())
	}
	defer conn.Close()

	wsr := newWebsocketReaderForLogs(conn, bool(ls.follow), ls.filter, ls.printer)
	return wsr.Listen(ctx)
}

// our websocket is protected by a csrf token, so we need to fetch it.
func (ls *LogStreamer) fetchWebsocketToken(ctx context.Context) (string, error) {
	tokenURL := ls.url
	tokenURL.Scheme = "http"
	tokenURL.Path = "/api/websocket_token"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set(server.TiltTokenHeaderName, token.Load())

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Wrapf(err, "connecting to Tilt at %s", tokenURL.String())
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", errors.Errorf("request to %s failed with status %q", tokenURL.String(), res.Status)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

type WebsocketConn interface {
	NextReader() (int, io.Reader, error)
	Close() error
	NextWriter(messageType int) (io.WriteCloser, error)
}

type WebsocketReader struct {
	conn       WebsocketConn
	persistent bool // whether to keep listening on websocket, or close after first message
	handler    ViewHandler
}

func newWebsocketReaderForLogs(conn WebsocketConn, persistent bool, filter LogFilter, printer LogPrinter) *WebsocketReader {
	ls := newLogViewHandler(filter, printer)
	return newWebsocketReader(conn, persistent, ls)
}

func newWebsocketReader(conn WebsocketConn, persistent bool, handler ViewHandler) *WebsocketReader {
	return &WebsocketReader{
		conn:       conn,
		persistent: persistent,
		handler:    handler,
	}
}

type ViewHandler interface {
	Handle(v *proto_webview.View) error
}

type logViewHandler struct {
	logstore *logstore.LogStore
	// checkpoint tracks the client's latest printed logs.
	//
	// WARNING: The server watermark values CANNOT be used for checkpointing within the client!
	checkpoint logstore.Checkpoint
	// serverWatermark ensures that we don't print any duplicate logs.
	//
	// This value should only be used to compare to other server values, NOT client checkpoints.
	serverWatermark int32
	filter          LogFilter
	printer         LogPrinter
	// isFirstBatch tracks whether we've received the first batch of logs.
	// Tail limit only applies to the first batch (initial history).
	isFirstBatch bool
}

func newLogViewHandler(filter LogFilter, p LogPrinter) *logViewHandler {
	return &logViewHandler{
		filter:       filter,
		logstore:     logstore.NewLogStore(),
		printer:      p,
		isFirstBatch: true,
	}
}

func (ls *logViewHandler) Handle(v *proto_webview.View) error {
	if v == nil || v.LogList == nil || v.LogList.FromCheckpoint == -1 {
		// Server has no new logs to send.
		// Mark first batch as processed so --tail doesn't apply to future logs.
		ls.isFirstBatch = false
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

	// Apply tail limit only on the first batch (initial history).
	// Subsequent batches in follow mode should show all new logs.
	if ls.isFirstBatch {
		lines = ls.filter.Apply(lines)
		ls.isFirstBatch = false
	} else {
		lines = ls.filter.ApplyWithoutTail(lines)
	}

	ls.printer.Print(lines)

	ls.checkpoint = ls.logstore.Checkpoint()
	ls.serverWatermark = v.LogList.ToCheckpoint

	return nil
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
	err := json.NewDecoder(reader).Decode(v)
	if err != nil {
		return errors.Wrap(err, "parsing")
	}

	err = wsr.handler.Handle(v)
	if err != nil {
		return errors.Wrap(err, "handling")
	}

	return nil
}
