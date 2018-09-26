package build

import (
	"fmt"
	"strings"

	digest "github.com/opencontainers/go-digest"
	"github.com/windmilleng/tilt/internal/logger"
)

type buildkitPrinter struct {
	logger logger.Logger
	vData  map[digest.Digest]*vertexAndLogs
	vOrder []digest.Digest
}

type vertex struct {
	digest      digest.Digest
	name        string
	error       string
	started     bool
	completed   bool
	cmdPrinted  bool
	logsPrinted bool
}

// HACK: The prefix we assume here isn't valid for RUNS in exec format
// Ex: RUN ["echo", "hello world"]
const cmdPrefix = "/bin/sh -c "
const buildPrefix = "    ╎ "

func (v *vertex) isRun() bool {
	return strings.HasPrefix(v.name, cmdPrefix)
}

func (v *vertex) isError() bool {
	return len(v.error) > 0
}

type vertexAndLogs struct {
	vertex *vertex
	logs   []*vertexLog
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
		if vl.vertex.isRun() && vl.vertex.started && !vl.vertex.cmdPrinted {
			b.logger.Infof("%sRUNNING: %s", buildPrefix, trimCmd(vl.vertex.name))
			vl.vertex.cmdPrinted = true
		}

		logWriter := b.logger.Writer(logger.VerboseLvl)
		if vl.vertex.isError() {
			b.logger.Infof("\n%sERROR IN: %s", buildPrefix, trimCmd(vl.vertex.name))
			logWriter = b.logger.Writer(logger.InfoLvl)
		}

		if vl.vertex.isRun() && vl.vertex.completed && !vl.vertex.logsPrinted {
			for _, l := range vl.logs {
				sl := strings.TrimSpace(string(l.msg))
				if len(sl) == 0 {
					continue
				}
				msg := fmt.Sprintf("%s  → %s\n", buildPrefix, sl)
				_, err := logWriter.Write([]byte(msg))
				if err != nil {
					return err
				}
			}
			vl.vertex.logsPrinted = true
		}
	}

	return nil
}

func trimCmd(cmd string) string {
	return strings.TrimPrefix(cmd, cmdPrefix)
}
