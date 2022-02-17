package filteredwriter

import (
	"io"
	"sync"
)

type filteredWriter struct {
	underlying io.Writer
	filterFunc func(s string) bool
	leftover   []byte
	mu         sync.Mutex
}

func (fw *filteredWriter) Write(buf []byte) (int, error) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	buf = append([]byte{}, append(fw.leftover, buf...)...)
	start := 0
	written := 0
	for i, b := range buf {
		if b == '\n' {
			end := i
			if buf[i-1] == '\r' {
				end--
			}
			s := string(buf[start:end])

			if !fw.filterFunc(s) {
				n, err := fw.underlying.Write(buf[start : i+1])
				written += n
				if err != nil {
					fw.leftover = append([]byte{}, buf[i+1:]...)
					return len(buf), err
				}
			}

			start = i + 1
		}
	}

	fw.leftover = append([]byte{}, buf[start:]...)

	return len(buf), nil
}

// lines matching `filterFunc` will not be output to the underlying writer
func New(underlying io.Writer, filterFunc func(s string) bool) io.Writer {
	return &filteredWriter{underlying: underlying, filterFunc: filterFunc}
}
