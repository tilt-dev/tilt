package build

import (
	"fmt"
	"io"
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
	digest     digest.Digest
	name       string
	error      string
	started    bool
	completed  bool
	cmdPrinted bool
}

const cmdPrefix = "/bin/sh -c "
const internalPrefix = "[internal]"
const buildPrefix = "    ╎ "

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
		if vl.vertex.started && !vl.vertex.cmdPrinted {
			if vl.vertex.isShellRun() {
				b.logger.Infof("%sRUNNING: %s", buildPrefix, trimCmd(vl.vertex.name))
				vl.vertex.cmdPrinted = true
			} else if !vl.vertex.isInternal() {
				b.logger.Infof("%s%s", buildPrefix, trimCmd(vl.vertex.name))
				vl.vertex.cmdPrinted = true
			}
		}

		if vl.vertex.isError() {
			b.logger.Infof("\n%sERROR IN: %s", buildPrefix, trimCmd(vl.vertex.name))
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

func trimCmd(cmd string) string {
	return strings.TrimPrefix(cmd, cmdPrefix)
}
