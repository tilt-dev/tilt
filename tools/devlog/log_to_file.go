// A logger that writes to a tmp file. For use in development only
// (presumably for HUD development).

package devlog

import (
	"fmt"
	"os"
	"sync"
)

var filename = "/tmp/tilt-log"
var theLogfile = logFile{mu: new(sync.Mutex)}

type logFile struct {
	f  *os.File
	mu *sync.Mutex
}

func (lf *logFile) write(s string) {
	lf.maybeLoad()

	_, err := lf.f.WriteString(s)
	if err != nil {
		panic(fmt.Sprintf("writing to logfile %s: %v", lf.f.Name(), err))
	}
}

func (lf *logFile) maybeLoad() {
	if lf.f != nil {
		return
	}

	lf.mu.Lock()
	defer lf.mu.Unlock()

	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		panic(fmt.Sprintf("creating logfile: %v", err))
	}

	lf.f = f
}

func Logf(msg string, a ...interface{}) {
	s := fmt.Sprintf(msg, a...)
	theLogfile.write(s + "\n")

}
