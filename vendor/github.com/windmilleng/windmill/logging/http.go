package logging

import (
	"net/http"

	http_logrus "github.com/improbable-eng/go-httpwares/logging/logrus"
)

func LoggingHandler(handler http.Handler) http.Handler {
	return http_logrus.Middleware(Global())(handler)
}
