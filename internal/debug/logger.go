package debug

import (
	"fmt"
)

var debugMode bool

func DebugLog(format string, a ...interface{}) {
	if debugMode {
		fmt.Printf(format, a...)
	}
}

func SetDebugMode(mode bool) {
	debugMode = mode
}
