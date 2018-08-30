package build

import "io"

type buildkitPrinter struct {
	output io.Writer
}

func newBuildkitPrinter(output io.Writer) buildkitPrinter {
	return buildkitPrinter{
		output: output,
	}
}

func (b *buildkitPrinter) parseAndPrint(vertexes []*vertex, logs []*log) error {

}
