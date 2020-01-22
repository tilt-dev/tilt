package build

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	digest "github.com/opencontainers/go-digest"

	"github.com/windmilleng/tilt/pkg/logger"
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
	errorPrinted    bool
	completePrinted bool
	cached          bool
	duration        time.Duration
}

const internalPrefix = "[internal]"
const logPrefix = "  → "

var stageNameRegexp = regexp.MustCompile(`^\[.+\]`)

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
	logger      logger.Logger
}

type vertexLog struct {
	vertex digest.Digest
	msg    []byte
}

func newBuildkitPrinter(l logger.Logger) *buildkitPrinter {
	return &buildkitPrinter{
		logger: l,
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
				logger: logger.NewPrefixedLogger(logPrefix, b.logger),
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
			b.logger.Infof("%s%s", cachePrefix, vl.vertex.name)
			vl.vertex.startPrinted = true
		}

		if vl.vertex.isError() && !vl.vertex.errorPrinted {
			// TODO(nick): Should this be logger.Errorf?
			b.logger.Infof("\nERROR IN: %s", vl.vertex.name)
			vl.vertex.errorPrinted = true
		}

		if vl.vertex.isError() || !vl.vertex.isInternal() {
			b.flushLogs(vl)
		}

		if vl.vertex.completed &&
			!vl.vertex.completePrinted &&
			!vl.vertex.isInternal() &&
			!vl.vertex.cached &&
			vl.vertex.duration > 200*time.Millisecond &&
			!vl.vertex.isError() {

			// TODO(nick): Should this use the progress API once it's available?
			b.logger.Infof("%s done | %s",
				vl.vertex.stageName(),
				vl.vertex.duration.Truncate(time.Millisecond))
			vl.vertex.completePrinted = true
		}
	}

	return nil
}

func (b *buildkitPrinter) flushLogs(vl *vertexAndLogs) {
	for vl.logsPrinted < len(vl.logs) {
		l := vl.logs[vl.logsPrinted]
		vl.logsPrinted++
		vl.logger.Write(logger.InfoLvl, []byte(l.msg))
	}
}
