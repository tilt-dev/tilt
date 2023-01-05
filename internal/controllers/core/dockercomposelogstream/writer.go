package dockercomposelogstream

import (
	"bytes"
	"time"
	"unicode"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/dockercomposeservices"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type LogActionWriter struct {
	store        store.RStore
	manifestName model.ManifestName

	attachMessageSeen bool

	since    time.Time
	lastTime time.Time
}

var newlineAsBytes = []byte("\n")
var attachingToLogAsBytes = []byte("Attaching to ")
var spaceAsBytes = []byte(" ")

func (w *LogActionWriter) Write(p []byte) (n int, err error) {
	lines := bytes.Split(p, newlineAsBytes)
	if !w.attachMessageSeen {
		if len(lines) != 0 && bytes.HasPrefix(lines[0], attachingToLogAsBytes) {
			lines = lines[1:]
			w.attachMessageSeen = true
		}
	}

	linesToWrite := make([][]byte, 0, len(lines))
	for _, line := range lines {
		hasTimestamp, timestamp, logContent := splitLogLineTimestamp(line)
		if hasTimestamp {
			// use version of the log line w/o the timestamp
			line = logContent
			w.lastTime = timestamp
			if !timestamp.After(w.since) {
				continue
			}
		}
		linesToWrite = append(linesToWrite, line)
	}

	if len(linesToWrite) == 0 {
		return len(p), nil
	}

	newText := bytes.Join(linesToWrite, newlineAsBytes)

	w.store.Dispatch(store.NewLogAction(w.manifestName,
		dockercomposeservices.SpanIDForDCService(w.manifestName), logger.InfoLvl, nil, newText))
	return len(p), nil
}

// LastLogTime returns the timestamp of the last log message seen or zero time if none.
//
// The last log message seen timestamp might be before the `since` argument, so was discarded.
//
// This method is not goroutine-safe: it is intended to be used after the writer is done.
func (w *LogActionWriter) LastLogTime() time.Time {
	return w.lastTime
}

// splitLogLineTimestamp attempts to extract a timestamp from a Docker Compose log line.
//
// Tilt invokes `docker-compose logs` with `--timestamps`, which will output timestamps in the RFC3339Nano format
// at the beginning of each log line with a single space as a divider afterwards. For example, if the container
// logs "Hello World\n", the output would be:
//
//	2021-09-08T18:24:24.704836400Z Hello World
//
// Unfortunately, there are caveats:
//   - docker-compose v2 prepends whitespace _before_ the timestamp as well
//   - Messages originating from docker-compose itself (e.g. container lifecycle messages) do NOT get a timestamp, e.g.
//     myproject_my-container_1 exited with code 0
//
// As a result, this function tries to be very conservative in extracting the timestamp.
func splitLogLineTimestamp(line []byte) (bool, time.Time, []byte) {
	if len(line) < 2 {
		return false, time.Time{}, nil
	}
	// docker-compose v2 prepends a space to every log line
	if unicode.IsSpace(rune(line[0])) {
		line = bytes.TrimLeftFunc(line, unicode.IsSpace)
		if len(line) == 0 {
			// in case we trim the whole line
			return false, time.Time{}, nil
		}
	}

	// docker-compose emits meta-logs about container events that don't start with a timestamp
	// N.B. it's actually possible for them to start with a number if the project name (typically dir name)
	// 	starts with a number, but that's very unlikely so this is used as a short-circuit for the common
	//  case to avoid attempting a parse that's guaranteed to fail, but it will still fail gracefully later
	if !unicode.IsDigit(rune(line[0])) {
		return false, time.Time{}, nil
	}

	index := bytes.Index(line, spaceAsBytes)
	if index == -1 {
		return false, time.Time{}, nil
	}

	logTimestamp, err := time.Parse(time.RFC3339Nano, string(line[:index]))
	if err != nil {
		return false, time.Time{}, nil
	}

	return true, logTimestamp, line[index+len(spaceAsBytes):]
}
