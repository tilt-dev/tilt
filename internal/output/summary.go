package output

import (
	"fmt"
	"io"
	"strconv"
)

type summary struct {
	output      io.Writer
	steps       string
	directories []*directory
}

type directory struct {
	name    string
	updated bool
}

func newSummary(output io.Writer) *summary {
	return &summary{
		output:      output,
		steps:       "",
		directories: []*directory{},
	}
}

func (s *summary) parse() {
	s.steps = strconv.Itoa(3)
}

func (s *summary) print() {
	s.parse()

	msg := fmt.Sprintf("Steps: %s\n", s.steps)
	s.output.Write([]byte(msg))
}
