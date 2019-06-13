package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/windmilleng/windmill/logging"
	"github.com/windmilleng/windmill/stats"
)

const keyTime = "time"

// A small http server that decodes json and sends it to our metrics services
type StatsServer struct {
	router   *mux.Router
	reporter stats.StatsReporter
}

func NewStatsServer(stats stats.StatsReporter) StatsServer {
	r := mux.NewRouter().UseEncodedPath()

	s := StatsServer{router: r, reporter: stats}
	r.HandleFunc("/report", s.Report).Methods("POST")
	r.HandleFunc("/", s.Index).Methods("GET")
	return s
}

func (s StatsServer) Router() *mux.Router {
	return s.router
}

func (s StatsServer) Index(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("OK"))
}

func (s StatsServer) Report(w http.ResponseWriter, r *http.Request) {
	now := time.Now()

	events, err := parseJSON(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("JSON decode: %v", err), http.StatusBadRequest)
		return
	}

	logger := logging.With(r.Context())

	for _, event := range events {
		name := event.Name()
		if name == "" {
			http.Error(w, fmt.Sprintf("Missing name: %v", event), http.StatusBadRequest)
			return
		}

		event[keyTime] = now.Format(time.RFC3339)

		dur := event.Duration()
		if dur == 0 {
			// TODO: support count

			err := s.reporter.Incr(name, event.Tags(), 1)
			if err != nil {
				logging.With(r.Context()).Warnf("Report error: %v", err)
			}
			logger.WithFields(event.LogFields()).Info("event")
		} else {
			err := s.reporter.Timing(name, dur, event.Tags(), 1)
			if err != nil {
				logging.With(r.Context()).Warnf("Report error: %v", err)
			}
			logger.WithFields(event.LogFields()).Info("timing")
		}

	}
}

type Event map[string]interface{}

func (e Event) Name() string {
	name, ok := e["name"].(string)
	if !ok {
		return ""
	}
	return name
}

func (e Event) Duration() time.Duration {
	// all json numbers are float64
	dur, ok := e["duration"].(float64)
	if !ok {
		return 0
	}
	return time.Duration(dur)
}

func (e Event) Tags() map[string]string {
	tags := make(map[string]string)
	for k, v := range e {
		if k == "name" || k == "duration" {
			continue
		}
		if vStr, ok := v.(string); ok {
			tags[k] = vStr
		}
	}
	return tags
}

func (e Event) LogFields() logrus.Fields {
	fields := logrus.Fields{}
	for k, v := range e {
		fields[k] = v
	}
	return fields
}

func parseJSON(r *http.Request) ([]Event, error) {
	d := json.NewDecoder(r.Body)
	result := make([]Event, 0)
	for d.More() {
		data := make(map[string]interface{})
		err := d.Decode(&data)
		if err != nil {
			return nil, err
		}

		result = append(result, Event(data))
	}
	return result, nil
}
