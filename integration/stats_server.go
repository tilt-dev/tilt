package integration

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/windmilleng/windmill/stats"
	"github.com/windmilleng/windmill/stats/server"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

type MemoryStatsServer struct {
	ma       *analytics.MemoryAnalytics
	ss       server.StatsServer
	listener net.Listener
}

func StartMemoryStatsServer() (mss *MemoryStatsServer, port int, err error) {
	mss = &MemoryStatsServer{}
	mss.ma = analytics.NewMemoryAnalytics()
	sr := &MemoryStatsReporter{mss.ma}
	mss.ss = server.NewStatsServer(sr)

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

var _ stats.StatsReporter = &MemoryStatsReporter{}

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
