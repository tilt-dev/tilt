package build

import (
	"fmt"
	"io"
	"strings"

	digest "github.com/opencontainers/go-digest"
)

type buildkitPrinter struct {
	output io.Writer
}

type vertex struct {
	digest digest.Digest
	name   string
	error  string
}

type vertexLog struct {
	vertex digest.Digest
	msg    []byte
}

type vertexAndLogs struct {
	vertex *vertex
	logs   []*vertexLog
}

func newBuildkitPrinter(output io.Writer) buildkitPrinter {
	return buildkitPrinter{
		output: output,
	}
}

func (b *buildkitPrinter) parseAndPrint(vertexes []*vertex, logs []*vertexLog) error {
	vMap := map[digest.Digest]vertexAndLogs{}

	for _, v := range vertexes {
		vMap[v.digest] = vertexAndLogs{
			vertex: v,
		}
	}

	for _, l := range logs {
		if val, ok := vMap[l.vertex]; ok {
			if len(l.msg) > 0 {
				vMap[l.vertex] = vertexAndLogs{
					vertex: val.vertex,
					logs:   append(val.logs, l),
				}
			}
		}
	}

	for _, v := range vMap {
		cmdPrefix := "/bin/sh -c "

		if !strings.HasPrefix(v.vertex.name, cmdPrefix) {
			continue
		}

		msg := fmt.Sprintf("RUN: %s\n", strings.TrimPrefix(v.vertex.name, cmdPrefix))
		b.output.Write([]byte(msg))

		if v.vertex.error != "" {
			for _, l := range v.logs {
				if len(l.msg) > 0 {
					msg := fmt.Sprintf("  → ERROR: %s\n", l.msg)
					b.output.Write([]byte(msg))
				}
			}
		}
	}

	return nil
}
