package env

import (
	"os"
)

func IsDebug() bool {
	return os.Getenv("WMDEBUG") != ""
}
