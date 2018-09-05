package build

import (
	"fmt"
	"io"
	"strings"

	digest "github.com/opencontainers/go-digest"
)

type buildkitPrinter struct {
	output io.Writer
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

func newBuildkitPrinter(output io.Writer) *buildkitPrinter {
	return &buildkitPrinter{
		output: output,
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
		if vl, ok := b.vData[d]; ok {
			if vl.vertex.isRun() && vl.vertex.started && !vl.vertex.cmdPrinted {
				msg := fmt.Sprintf("%sRUNNING: %s\n", buildPrefix, trimCmd(vl.vertex.name))
				_, err := b.output.Write([]byte(msg))
				if err != nil {
					return err
				}

				vl.vertex.cmdPrinted = true
			}

			if vl.vertex.isError() {
				msg := fmt.Sprintf("\n%sERROR IN: %s\n", buildPrefix, trimCmd(vl.vertex.name))
				_, err := b.output.Write([]byte(msg))
				if err != nil {
					return err
				}

				for _, l := range vl.logs {
					sl := strings.TrimSpace(string(l.msg))
					if len(sl) == 0 {
						continue
					}
					msg := fmt.Sprintf("%s  → %s\n", buildPrefix, sl)
					_, err := b.output.Write([]byte(msg))
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func trimCmd(cmd string) string {
	return strings.TrimPrefix(cmd, cmdPrefix)
}
