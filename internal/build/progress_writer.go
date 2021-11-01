package build

import (
	"context"
	"io"
	"time"

	"github.com/docker/go-units"

	"github.com/tilt-dev/tilt/pkg/logger"
)

// A little utility class that tracks how many bytes we've written,
// to the Docker context.
type ProgressWriter struct {
	ctx                context.Context
	delegate           io.Writer
	createTime         time.Time
	byteCount          int
	lastPrintTime      time.Time
	lastPrintByteCount int
}

func NewProgressWriter(ctx context.Context, w io.Writer) *ProgressWriter {
	return &ProgressWriter{
		ctx:        ctx,
		delegate:   w,
		createTime: time.Now(),
	}
}

func (w *ProgressWriter) Write(b []byte) (int, error) {
	// The io.Writer API can handle partial writes,
	// so write first, then print the results of the Write.
	n, err := w.delegate.Write(b)

	w.byteCount += n

	hasBeenPrinted := !w.lastPrintTime.IsZero()
	shouldPrint := !hasBeenPrinted ||
		time.Since(w.lastPrintTime) > 2*time.Second ||
		w.byteCount > 2*w.lastPrintByteCount
	if shouldPrint {
		w.info(logger.Fields{})
		w.lastPrintTime = time.Now()
		w.lastPrintByteCount = w.byteCount
	}

	return n, err
}

func (w *ProgressWriter) Init() {
	w.info(logger.Fields{})
}

func (w *ProgressWriter) Close() {
	fields := logger.Fields{
		logger.FieldNameProgressMustPrint: "1",
	}
	w.info(fields)
}

func (w *ProgressWriter) info(fields logger.Fields) {
	fields[logger.FieldNameProgressID] = "tilt-context-upload"
	logger.Get(w.ctx).WithFields(fields).
		Infof("Sending Docker build context: %s (%s)",
			units.HumanSize(float64(w.byteCount)),
			time.Since(w.createTime).Truncate(time.Millisecond))
}
