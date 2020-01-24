// A logger that writes to a tmp file. For use in development only
// (presumably for HUD development).

package devlog

import (
	"fmt"
	"os"
	"sync"
	"time"
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

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(fmt.Sprintf("creating logfile: %v", err))
	}

	lf.f = f
}

// THIS SHOULD NEVER END UP IN IN PRODUCTION CODE. This func is for debugging
// when it's awkward to Printf/use the real logger. Calls to this func should
// never end up on `master.`
func Logf(msg string, a ...interface{}) {
	s := fmt.Sprintf("%s: %s", time.Now().Format(time.RFC3339), fmt.Sprintf(msg, a...))
	theLogfile.write(s + "\n")

}
