// A logger that writes to a tmp file. For use in development only
// (presumably for HUD development).

package devlog

import (
	"fmt"
	"os"
	"time"
)

var logfileBaseName = "/tmp/tilt-log-%s"
var theLogfile *os.File // THERE CAN ONLY BE ONE!

func logFileName() string {
	now := time.Now()
	tStr := now.Format("2006-01-02T15.04.05")
	return fmt.Sprintf(logfileBaseName, tStr)
}

func LogToFilef(msg string, a ...interface{}) {
	var err error

	if theLogfile == nil {
		logfile := logFileName()
		theLogfile, err = os.Create(logfile)
		if err != nil {
			panic(fmt.Sprintf("creating logfile %s: %v", logfile, err))
		}
	}

	s := fmt.Sprintf(msg, a...)
	_, err = theLogfile.WriteString(s)
	if err != nil {
		panic(fmt.Sprintf("writing to logfile %s: %v", theLogfile.Name(), err))
	}
}
