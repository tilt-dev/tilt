package build

import (
	"fmt"
	"io"
	"strings"

	digest "github.com/opencontainers/go-digest"
)

type buildkitPrinter struct {
	output io.Writer
	vMap   map[digest.Digest]*vertexAndLogs
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

var logDigest digest.Digest

func newBuildkitPrinter(output io.Writer) *buildkitPrinter {
	return &buildkitPrinter{
		output: output,
		vMap:   map[digest.Digest]*vertexAndLogs{},
	}
}

func (b *buildkitPrinter) parse(vertexes []*vertex, logs []*vertexLog) {

	for _, v := range vertexes {
		if _, ok := b.vMap[v.digest]; ok {
			continue
		}

		b.vMap[v.digest] = &vertexAndLogs{
			vertex: v,
			logs:   []*vertexLog{},
		}
	}

	for _, l := range logs {
		if _, ok := b.vMap[l.vertex]; ok {
			logDigest = l.vertex
			if len(l.msg) > 0 {
				vl := b.vMap[l.vertex]
				vl.logs = append(vl.logs, l)
			}
		}
	}
}

func (b *buildkitPrinter) print() error {
	for _, v := range b.vMap {
		buildPrefix := "    ╎ "
		cmdPrefix := "/bin/sh -c "
		name := strings.TrimPrefix(v.vertex.name, cmdPrefix)

		for _, l := range v.logs {
			if len(l.msg) > 0 {
				msg := fmt.Sprintf("%s  → ERROR: %s\n", buildPrefix, l.msg)
				_, err := b.output.Write([]byte(msg))
				if err != nil {
					return err
				}
			}
		}

		if !strings.HasPrefix(v.vertex.name, cmdPrefix) {
			continue
		}

		msg := fmt.Sprintf("%sRUN: %s\n", buildPrefix, name)
		_, err := b.output.Write([]byte(msg))
		if err != nil {
			return err
		}
	}

	return nil
}
