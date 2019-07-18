package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

type MemoryStatsServer struct {
	ma       *analytics.MemoryAnalytics
	ss       StatsServer
	listener net.Listener
}

func StartMemoryStatsServer() (mss *MemoryStatsServer, port int, err error) {
	mss = &MemoryStatsServer{}
	mss.ma = analytics.NewMemoryAnalytics()
	sr := &MemoryStatsReporter{mss.ma}
	mss.ss = NewStatsServer(sr)

	mux := http.NewServeMux()
	mux.Handle("/", mss.ss.Router())

	mss.listener, err = net.Listen("tcp", ":0")
	if err != nil {
		return nil, 0, err
	}

	go func() {
		err := http.Serve(mss.listener, mux)
		if err != nil {
			fmt.Println(err)
		}
	}()

	return mss, mss.listener.Addr().(*net.TCPAddr).Port, nil
}

func (mss *MemoryStatsServer) TearDown() error {
	return mss.listener.Close()
}

type MemoryStatsReporter struct {
	a *analytics.MemoryAnalytics
}

var _ StatsReporter = &MemoryStatsReporter{}

func (sr *MemoryStatsReporter) Close() error {
	return nil
}

func (sr *MemoryStatsReporter) Timing(name string, value time.Duration, tags map[string]string, rate float64) error {
	sr.a.Timer(name, value, tags)
	return nil
}

func (sr *MemoryStatsReporter) Count(name string, value int64, tags map[string]string, rate float64) error {
	sr.a.Count(name, tags, int(value))
	return nil
}

func (sr *MemoryStatsReporter) Incr(name string, tags map[string]string, rate float64) error {
	sr.a.Incr(name, tags)
	return nil
}

const keyTime = "time"

type StatsReporter interface {
	io.Closer
	Timing(name string, value time.Duration, tags map[string]string, rate float64) error
	Count(name string, value int64, tags map[string]string, rate float64) error
	Incr(name string, tags map[string]string, rate float64) error
}

// A small http server that decodes json and sends it to our metrics services
type StatsServer struct {
	router   *mux.Router
	reporter StatsReporter
}

func NewStatsServer(stats StatsReporter) StatsServer {
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
				log.Printf("Report error: %v", err)
			}
		} else {
			err := s.reporter.Timing(name, dur, event.Tags(), 1)
			if err != nil {
				log.Printf("Report error: %v", err)
			}
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
