package logger

import (
	"context"
	"sync"
)

// A logger that buffers its log lines until we have an output logger.
type DeferredLogger struct {
	Logger
	entries  []logEntry
	mu       sync.Mutex
	original Logger
	output   Logger
}

func NewDeferredLogger(ctx context.Context) *DeferredLogger {
	original := Get(ctx)
	dLogger := &DeferredLogger{original: original}
	fLogger := NewFuncLogger(original.SupportsColor(), original.Level(), func(level Level, fields Fields, b []byte) error {
		dLogger.mu.Lock()
		defer dLogger.mu.Unlock()
		if dLogger.output != nil {
			dLogger.output.Write(level, b)
			return nil
		}
		dLogger.entries = append(dLogger.entries, logEntry{level: level, b: append([]byte{}, b...)})
		return nil
	})
	dLogger.Logger = fLogger
	return dLogger
}

// Set the output logger, and send all the buffered output to the new logger.
func (dl *DeferredLogger) SetOutput(l Logger) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.output = l
	for _, entry := range dl.entries {
		dl.output.Write(entry.level, entry.b)
	}
	dl.entries = nil
}

// The original logger that we're deferring output away from.
func (dl *DeferredLogger) Original() Logger {
	return dl.original
}

type logEntry struct {
	level Level
	b     []byte
}
