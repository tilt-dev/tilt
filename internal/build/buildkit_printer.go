package build

import (
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
}

type vertexLog struct {
	vertex digest.Digest
	msg    []byte
}

func newBuildkitPrinter(output io.Writer) buildkitPrinter {
	return buildkitPrinter{
		output: output,
	}
}

func (b *buildkitPrinter) parseAndPrint(vertexes []*vertex, logs []*vertexLog) error {

	for _, v := range vertexes {
		b.output.Write([]byte(strings.Replace(v.name, "/bin/sh -c", "RUN:", 1)))
	}

	for _, l := range logs {
		b.output.Write([]byte("\n→ ERROR: "))
		b.output.Write(l.msg)
	}

	return nil
}
