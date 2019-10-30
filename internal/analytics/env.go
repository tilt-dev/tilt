package analytics

import "os"

const disableAnalyticsEnvVar = "TILT_DISABLE_ANALYTICS"

func IsAnalyticsDisabledFromEnv() bool {
	return os.Getenv(disableAnalyticsEnvVar) != ""
}
