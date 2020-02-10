package build

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	digest "github.com/opencontainers/go-digest"
	"github.com/tonistiigi/units"

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
	durationPrinted time.Duration
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

	// A map of statuses, indexed by the layer id being downloaded.
	statuses vertexStatusSet

	// A combined status for all the downloadable layers, merged
	// into a single status object.
	lastPrintedStatus vertexStatus
}

type vertexLog struct {
	vertex digest.Digest
	msg    []byte
}

type vertexStatus struct {
	vertex    digest.Digest
	id        string
	total     int64
	current   int64
	timestamp time.Time
}

// The buildkit protocol represents each downloadable layer
// as a separate status object, identified by a layer ID.
// We want to present this to the user as a single, combined status
// that summarizes all layers.
type vertexStatusSet map[string]vertexStatus

func (s vertexStatusSet) combined() vertexStatus {
	current := int64(0)
	total := int64(0)
	t := time.Time{}
	for _, v := range s {
		current += v.current
		total += v.total
		if v.timestamp.After(t) {
			t = v.timestamp
		}
	}
	return vertexStatus{
		current:   current,
		total:     total,
		timestamp: t,
	}
}

func newBuildkitPrinter(l logger.Logger) *buildkitPrinter {
	return &buildkitPrinter{
		logger: l,
		vData:  map[digest.Digest]*vertexAndLogs{},
		vOrder: []digest.Digest{},
	}
}

func (b *buildkitPrinter) parseAndPrint(vertexes []*vertex, logs []*vertexLog, statuses []*vertexStatus) error {
	for _, v := range vertexes {
		if vl, ok := b.vData[v.digest]; ok {
			vl.vertex.started = v.started
			vl.vertex.completed = v.completed
			vl.vertex.cached = v.cached

			// NOTE(nick): Fun fact! The buildkit protocol sends down multiple completion timestamps.
			// We need to take the last one.
			if v.duration > vl.vertex.duration {
				vl.vertex.duration = v.duration
			}

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

	for _, s := range statuses {
		if vl, ok := b.vData[s.vertex]; ok {
			if vl.statuses == nil {
				vl.statuses = vertexStatusSet{}
			}
			vl.statuses[s.id] = *s
		}
	}

	for _, d := range b.vOrder {
		vl, ok := b.vData[d]
		if !ok {
			return fmt.Errorf("Expected to find digest %s in %+v", d, b.vData)
		}

		v := vl.vertex
		if v.started && !v.startPrinted && !v.isInternal() {
			cacheSuffix := ""
			if v.cached {
				cacheSuffix = " [cached]"
			}
			b.logger.WithFields(logger.Fields{logger.FieldNameProgressID: v.stageName()}).
				Infof("%s%s", v.name, cacheSuffix)
			v.startPrinted = true
		}

		if v.isError() && !v.errorPrinted {
			// TODO(nick): Should this be logger.Errorf?
			b.logger.Infof("\nERROR IN: %s", v.name)
			v.errorPrinted = true
		}

		if v.isError() || !v.isInternal() {
			b.flushLogs(vl)
		}

		if !v.isInternal() &&
			!v.cached &&
			!v.isError() {

			var progressInBytes string
			status := vl.statuses.combined()
			shouldPrintProgress := false
			if vl.lastPrintedStatus.total != status.total {
				// print progress when the total has changed. That means we've started
				// downloading a new layer.
				shouldPrintProgress = true
			} else if status.total > 0 {
				// print progress when at least 1% has changed and at least 2 seconds have passed.
				diff := float64(status.current) - float64(vl.lastPrintedStatus.current)
				largeEnoughChange := diff/float64(status.total) >= 0.01
				largeEnoughTime := status.timestamp.Sub(vl.lastPrintedStatus.timestamp) > 2*time.Second
				shouldPrintProgress = largeEnoughChange && largeEnoughTime
			}

			if status.total != 0 {
				progressInBytes = fmt.Sprintf(" %.2f / %.2f", units.Bytes(status.current), units.Bytes(status.total))
			} else if status.current != 0 {
				progressInBytes = fmt.Sprintf(" %.2f", units.Bytes(status.current))
			}

			// NOTE(nick): Fun fact! The buildkit protocol sends down multiple completion timestamps.
			// We need to print the longest one.
			shouldPrintCompletion := v.completed && v.duration > 10*time.Millisecond &&
				(!v.completePrinted ||
					v.durationPrinted < v.duration)

			doneSuffix := ""
			fields := logger.Fields{logger.FieldNameProgressID: v.stageName()}
			if shouldPrintCompletion {
				doneSuffix = fmt.Sprintf(" [done: %s]", v.duration.Truncate(time.Millisecond))
				v.completePrinted = true
				v.durationPrinted = v.duration
				fields[logger.FieldNameProgressMustPrint] = "1"
			}

			if shouldPrintCompletion || shouldPrintProgress {
				b.logger.WithFields(fields).
					Infof("%s%s%s", v.name, progressInBytes, doneSuffix)

				vl.lastPrintedStatus = status
			}
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
