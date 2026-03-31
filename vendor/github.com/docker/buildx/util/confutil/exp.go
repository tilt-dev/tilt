package confutil

import (
	"os"
	"strconv"
)

// IsExperimental checks if the experimental flag has been configured.
func IsExperimental() bool {
	if v, ok := os.LookupEnv("BUILDX_EXPERIMENTAL"); ok {
		vv, _ := strconv.ParseBool(v)
		return vv
	}
	return false
}
