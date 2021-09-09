package runtimelog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"
	"unicode"

	"github.com/docker/docker/api/types"

	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

// Collects logs from running docker-compose services.
type DockerComposeLogManager struct {
	watches map[model.ManifestName]dockerComposeLogWatch
	dcc     dockercompose.DockerComposeClient
}

func NewDockerComposeLogManager(dcc dockercompose.DockerComposeClient) *DockerComposeLogManager {
	return &DockerComposeLogManager{
		watches: make(map[model.ManifestName]dockerComposeLogWatch),
		dcc:     dcc,
	}
}

// Diff the current watches against set of current docker-compose services, i.e.
// what we SHOULD be watching, returning the changes we need to make.
func (m *DockerComposeLogManager) diff(ctx context.Context, st store.RStore) (setup []dockerComposeLogWatch, teardown []dockerComposeLogWatch) {
	state := st.RLockState()
	defer st.RUnlockState()

	for _, mt := range state.ManifestTargets {
		manifest := mt.Manifest
		if !manifest.IsDC() {
			continue
		}

		// If the build hasn't started yet, don't start watching.
		//
		// TODO(nick): This points to a larger synchronization bug between DC
		// LogManager and BuildController. Starting a build will delete all the logs
		// that have been recorded so far. This creates race conditions: if the logs
		// come in before the StartBuild event is recorded, those logs will get
		// deleted. This affects tests more than normal builds.
		// But we should have a better way to associate logs with a particular build.
		ms := mt.State
		if ms.CurrentBuild.StartTime.IsZero() && ms.LastBuild().StartTime.IsZero() {
			continue
		}
		dcState := ms.DCRuntimeState()
		if dcState.ContainerState.StartedAt == "" {
			// wait for the container to start before attaching so that we can filter out old logs
			// for job containers that can be re-used
			continue
		}

		// Docker evidently records the container start time asynchronously, so it can actually be AFTER
		// the first log timestamps (also reported by Docker), so we pad it by a second to reduce the
		// number of potentially duplicative logs
		startWatchTime := containerStartTime(dcState.ContainerState).Add(-time.Second)
		existing, hasExisting := m.watches[manifest.Name]
		if hasExisting {
			if !existing.Done() {
				// watcher is already running
				continue
			}

			if !existing.startWatchTime.Before(startWatchTime) {
				// watcher finished but the container hasn't started up again
				// (N.B. we cannot compare on the container ID because containers can restart and be re-used
				// 	after being stopped for jobs that run to completion but are re-triggered)
				continue
			}
		}

		ctx, cancel := context.WithCancel(ctx)
		w := dockerComposeLogWatch{
			ctx:            ctx,
			cancel:         cancel,
			name:           manifest.Name,
			dc:             manifest.DockerComposeTarget(),
			startWatchTime: startWatchTime,
		}
		m.watches[manifest.Name] = w
		setup = append(setup, w)
	}

	for key, value := range m.watches {
		_, inState := state.ManifestTargets[key]
		if !inState {
			delete(m.watches, key)

			teardown = append(teardown, value)
		}
	}

	return setup, teardown
}

func (m *DockerComposeLogManager) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) error {
	if summary.IsLogOnly() {
		return nil
	}

	setup, teardown := m.diff(ctx, st)
	for _, watch := range teardown {
		watch.cancel()
	}

	for _, watch := range setup {
		go m.consumeLogs(watch, st)
	}
	return nil
}

func (m *DockerComposeLogManager) consumeLogs(watch dockerComposeLogWatch, st store.RStore) {
	defer watch.cancel()

	startTime := watch.startWatchTime
	name := watch.name

	for {
		readCloser := m.dcc.StreamLogs(watch.ctx, watch.dc.ConfigPaths, watch.dc.Name)
		actionWriter := &DockerComposeLogActionWriter{
			store:        st,
			manifestName: name,
			since:        startTime,
		}

		_, err := io.Copy(actionWriter, readCloser)
		_ = readCloser.Close()
		if err == nil || watch.ctx.Err() != nil {
			// stop tailing because either:
			// 	* docker-compose logs exited naturally -> this means the container exited, so a new watcher will
			// 	  be created once a new container is seen
			//  * context was canceled -> manifest is no longer in engine & being torn-down
			return
		}

		// something went wrong with docker-compose, log it and re-attach, starting from the last
		// successfully logged timestamp
		logger.Get(watch.ctx).Debugf("Error streaming %s logs: %v", name, err)
		startTime = actionWriter.LastLogTime()
	}
}

type dockerComposeLogWatch struct {
	ctx            context.Context
	cancel         func()
	name           model.ManifestName
	dc             model.DockerComposeTarget
	startWatchTime time.Time
}

func (w *dockerComposeLogWatch) Done() bool {
	select {
	case <-w.ctx.Done():
		return true
	default:
		return false
	}
}

type DockerComposeLogActionWriter struct {
	store        store.RStore
	manifestName model.ManifestName

	attachMessageSeen bool

	since    time.Time
	lastTime time.Time
}

var newlineAsBytes = []byte("\n")
var attachingToLogAsBytes = []byte("Attaching to ")
var spaceAsBytes = []byte(" ")

func (w *DockerComposeLogActionWriter) Write(p []byte) (n int, err error) {
	lines := bytes.Split(p, newlineAsBytes)
	if !w.attachMessageSeen {
		if len(lines) != 0 && bytes.HasPrefix(lines[0], attachingToLogAsBytes) {
			lines = lines[1:]
			w.attachMessageSeen = true
		}
	}

	linesToWrite := make([][]byte, 0, len(lines))
	for _, line := range lines {
		hasTimestamp, timestamp, logContent := splitDockerComposeLogLineTimestamp(line)
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

	w.store.Dispatch(store.NewLogAction(w.manifestName, SpanIDForDCService(w.manifestName), logger.InfoLvl, nil, newText))
	return len(p), nil
}

// LastLogTime returns the timestamp of the last log message seen or zero time if none.
//
// The last log message seen timestamp might be before the `since` argument, so was discarded.
//
// This method is not goroutine-safe: it is intended to be used after the writer is done.
func (w *DockerComposeLogActionWriter) LastLogTime() time.Time {
	return w.lastTime
}

var _ store.Subscriber = &DockerComposeLogManager{}

func SpanIDForDCService(mn model.ManifestName) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("dc:%s", mn))
}

func containerStartTime(cs types.ContainerState) time.Time {
	if cs.StartedAt == "" {
		return time.Time{}
	}
	startTime, err := time.Parse(time.RFC3339Nano, cs.StartedAt)
	if err != nil {
		return time.Time{}
	}
	return startTime
}

// splitDockerComposeLogLineTimestamp attempts to extract a timestamp from a Docker Compose log line.
//
// Tilt invokes `docker-compose logs` with `--timestamps`, which will output timestamps in the RFC3339Nano format
// at the beginning of each log line with a single space as a divider afterwards. For example, if the container
// logs "Hello World\n", the output would be:
// 		2021-09-08T18:24:24.704836400Z Hello World
//
// Unfortunately, there are caveats:
// 	* docker-compose v2 prepends whitespace _before_ the timestamp as well
//  * Messages originating from docker-compose itself (e.g. container lifecycle messages) do NOT get a timestamp, e.g.
// 		myproject_my-container_1 exited with code 0
//
// As a result, this function tries to be very conservative in extracting the timestamp.
func splitDockerComposeLogLineTimestamp(line []byte) (bool, time.Time, []byte) {
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
