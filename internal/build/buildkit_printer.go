package build

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	digest "github.com/opencontainers/go-digest"

	"github.com/windmilleng/tilt/internal/logger"
)

type buildkitPrinter struct {
	logger logger.Logger
	vData  map[digest.Digest]*vertexAndLogs
	vOrder []digest.Digest
}

type vertex struct {
	digest          digest.Digest
	name            string
	error           string
	started         bool
	completed       bool
	startPrinted    bool
	completePrinted bool
	cached          bool
	duration        time.Duration
}

const cmdPrefix = "/bin/sh -c "
const internalPrefix = "[internal]"
const buildPrefix = "    ╎ "

var stageNameRegexp = regexp.MustCompile("^\\[.+\\]")

// TODO(nick): Delete isShellRun() and do this all differently.
// We shouldn't try to do this until we have a test framework for
// interpreting buildkit output.
func (v *vertex) isShellRun() bool {
	return strings.HasPrefix(v.name, cmdPrefix)
}

func (v *vertex) isInternal() bool {
	return strings.HasPrefix(v.name, internalPrefix)
}

func (v *vertex) isError() bool {
	return len(v.error) > 0
}

func (v *vertex) stageName() string {
	match := stageNameRegexp.FindString(v.name)
	if match == "" {
		// If we couldn't find a match, just return the whole
		// vertex name, so that the user has some hope of figuring out
		// what went wrong.
		return v.name
	}
	return match
}

type vertexAndLogs struct {
	vertex      *vertex
	logs        []*vertexLog
	logsPrinted int
}

type vertexLog struct {
	vertex digest.Digest
	msg    []byte
}

func newBuildkitPrinter(logger logger.Logger) *buildkitPrinter {
	return &buildkitPrinter{
		logger: logger,
		vData:  map[digest.Digest]*vertexAndLogs{},
		vOrder: []digest.Digest{},
	}
}

func (b *buildkitPrinter) parseAndPrint(vertexes []*vertex, logs []*vertexLog) error {
	for _, v := range vertexes {
		if vl, ok := b.vData[v.digest]; ok {
			vl.vertex.started = v.started
			vl.vertex.completed = v.completed
			vl.vertex.duration = v.duration
			vl.vertex.cached = v.cached

			if v.isError() {
				vl.vertex.error = v.error
			}
		} else {
			b.vData[v.digest] = &vertexAndLogs{
				vertex: v,
				logs:   []*vertexLog{},
			}

			b.vOrder = append(b.vOrder, v.digest)
		}
	}

	for _, l := range logs {
		if vl, ok := b.vData[l.vertex]; ok {
			vl.logs = append(vl.logs, l)
		}
	}

	for _, d := range b.vOrder {
		vl, ok := b.vData[d]
		if !ok {
			return fmt.Errorf("Expected to find digest %s in %+v", d, b.vData)
		}
		if vl.vertex.started && !vl.vertex.startPrinted && !vl.vertex.isInternal() {
			cachePrefix := ""
			if vl.vertex.cached {
				cachePrefix = "[cached] "
			}
			b.logger.Infof("%s%s%s", buildPrefix, cachePrefix, vl.vertex.name)
			vl.vertex.startPrinted = true
		}

		if vl.vertex.isError() {
			b.logger.Infof("\n%sERROR IN: %s", buildPrefix, vl.vertex.name)
			err := b.flushLogs(b.logger.Writer(logger.InfoLvl), vl)
			if err != nil {
				return err
			}
		}

		if !vl.vertex.isInternal() {
			err := b.flushLogs(b.logger.Writer(logger.InfoLvl), vl)
			if err != nil {
				return err
			}
		}

		if vl.vertex.completed &&
			!vl.vertex.completePrinted &&
			!vl.vertex.isInternal() &&
			!vl.vertex.cached &&
			vl.vertex.duration > 200*time.Millisecond &&
			!vl.vertex.isError() {
			b.logger.Infof("%s%s done | %s",
				buildPrefix, vl.vertex.stageName(),
				vl.vertex.duration.Truncate(time.Millisecond))
			vl.vertex.completePrinted = true
		}
	}

	return nil
}

func (b *buildkitPrinter) flushLogs(writer io.Writer, vl *vertexAndLogs) error {
	for vl.logsPrinted < len(vl.logs) {
		l := vl.logs[vl.logsPrinted]
		vl.logsPrinted++

		sl := strings.TrimSpace(string(l.msg))
		if len(sl) == 0 {
			continue
		}
		msg := fmt.Sprintf("%s  → %s\n", buildPrefix, sl)
		_, err := writer.Write([]byte(msg))
		if err != nil {
			return err
		}
	}
	return nil
}
