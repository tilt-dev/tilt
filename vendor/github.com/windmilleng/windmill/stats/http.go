package stats

// HTTP middleware for stats

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

func StatsHandler(handler http.Handler, router *mux.Router, reporter StatsReporter) http.Handler {
	return statsHandler{
		handler:  handler,
		router:   router,
		reporter: reporter,
	}
}

type statsHandler struct {
	handler  http.Handler
	router   *mux.Router
	reporter StatsReporter
}

type statsResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statsResponseWriter) Write(b []byte) (int, error) {
	w.status = http.StatusOK
	return w.ResponseWriter.Write(b)
}

func (w *statsResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (h statsHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	myRes := &statsResponseWriter{ResponseWriter: res}

	h.handler.ServeHTTP(myRes, req)

	var match mux.RouteMatch
	matched := h.router.Match(req, &match)
	routeName := ""
	routePattern := ""
	if matched && match.Route != nil {
		routeName = match.Route.GetName()
		routePattern, _ = match.Route.GetPathTemplate()
	}

	if routeName == "" {
		routeName = "unknown"
	}

	if routePattern == "" {
		routePattern = "unknown"
	}

	duration := time.Now().Sub(startTime)
	tags := map[string]string{
		"route": routeName,
		"pattern": routePattern,
		"status": strconv.Itoa(myRes.status),
	}

	h.reporter.Incr("http.response", tags, 1)
	h.reporter.Timing("http.responseTime", duration, tags, 1)
}
