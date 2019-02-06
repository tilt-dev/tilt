package analytics

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/denisbrodbeck/machineid"
)

const statsEndpt = "https://events.windmill.build/report"
const contentType = "Content-Type"
const contentTypeJson = "application/json"
const statsTimeout = time.Minute

// keys for request to stats server
const (
	keyDuration = "duration"
	keyName     = "name"
	keyUser     = "user"
	keyMachine  = "machine"
)

var cli = &http.Client{Timeout: statsTimeout}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Logger interface {
	Printf(format string, v ...interface{})
}

type stdLogger struct{}

func (stdLogger) Printf(format string, v ...interface{}) {
	log.Printf("[analytics] %s", fmt.Sprintf(format, v...))
}

func Init(appName string, options ...Option) (Analytics, *cobra.Command, error) {
	a, err := NewRemoteAnalytics(appName, options...)
	if err != nil {
		return nil, nil, err
	}

	c, err := initCLI()
	if err != nil {
		return nil, nil, err
	}

	return a, c, nil
}

type Analytics interface {
	Count(name string, tags map[string]string, n int)
	Incr(name string, tags map[string]string)
	Timer(name string, dur time.Duration, tags map[string]string)
	Flush(timeout time.Duration)
}

type remoteAnalytics struct {
	cli        HTTPClient
	app        string
	url        string
	enabled    bool
	logger     Logger
	globalTags map[string]string
	wg         *sync.WaitGroup
}

func hashMD5(in []byte) string {
	h := md5.New()

	// hash writes are guaranteed never to error
	_, _ = h.Write(in)

	return fmt.Sprintf("%x", h.Sum(nil))
}

// getUserHash returns a unique identifier for this user by hashing `uname -a`
func getUserID() string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	cmd := exec.CommandContext(ctx, "uname", "-a")
	out, err := cmd.Output()
	if err != nil || ctx.Err() != nil {
		// Something went wrong, but ¯\_(ツ)_/¯
		return "anon"
	}
	return hashMD5(out)
}

func getMachineID() string {
	mid, err := machineid.ID()
	if err != nil {
		return "anon"
	}
	return hashMD5([]byte(mid))
}

// Create a remote analytics object with Windmill-specific defaults
// for the HTTPClient, report URL, user ID, and opt-in status.
// All of these defaults can be overridden with appropriate options.
func NewRemoteAnalytics(appName string, options ...Option) (*remoteAnalytics, error) {
	enabled, err := optedIn()
	if err != nil {
		return nil, err
	}
	a := &remoteAnalytics{
		cli:        cli,
		app:        appName,
		url:        statsEndpt,
		enabled:    enabled,
		logger:     stdLogger{},
		wg:         &sync.WaitGroup{},
		globalTags: map[string]string{keyUser: getUserID(), keyMachine: getMachineID()},
	}
	for _, o := range options {
		o(a)
	}
	return a, nil
}

func (a *remoteAnalytics) namespaced(name string) string {
	return fmt.Sprintf("%s.%s", a.app, name)
}

func (a *remoteAnalytics) baseReqBody(name string, tags map[string]string) map[string]interface{} {
	req := map[string]interface{}{keyName: a.namespaced(name)}
	for k, v := range a.globalTags {
		req[k] = v
	}
	for k, v := range tags {
		req[k] = v
	}
	return req
}

func (a *remoteAnalytics) makeReq(reqBody map[string]interface{}) (*http.Request, error) {
	j, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal: %v\n", err)
	}
	reader := bytes.NewReader(j)

	req, err := http.NewRequest(http.MethodPost, a.url, reader)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest: %v\n", err)
	}
	req.Header.Add(contentType, contentTypeJson)

	return req, nil
}

func (a *remoteAnalytics) Count(name string, tags map[string]string, n int) {
	if !a.enabled {
		return
	}

	a.wg.Add(1)
	go a.count(name, tags, n)
}

func (a *remoteAnalytics) count(name string, tags map[string]string, n int) {
	defer a.wg.Done()

	req, err := a.countReq(name, tags, n)
	if err != nil {
		// Stat reporter can't return errs, just print it.
		a.logger.Printf("Error: %v\n", err)
		return
	}

	resp, err := a.cli.Do(req)
	if err != nil {
		a.logger.Printf("http.Post: %v\n", err)
		return
	}
	if resp.StatusCode != 200 {
		a.logger.Printf("http.Post returned status: %s\n", resp.Status)
	}
}

func (a *remoteAnalytics) countReq(name string, tags map[string]string, n int) (*http.Request, error) {
	// TODO: include n
	return a.makeReq(a.baseReqBody(name, tags))
}

func (a *remoteAnalytics) Incr(name string, tags map[string]string) {
	if !a.enabled {
		return
	}
	a.Count(name, tags, 1)
}

func (a *remoteAnalytics) Timer(name string, dur time.Duration, tags map[string]string) {
	if !a.enabled {
		return
	}

	a.wg.Add(1)
	go a.timer(name, dur, tags)
}

func (a *remoteAnalytics) timer(name string, dur time.Duration, tags map[string]string) {
	defer a.wg.Done()

	req, err := a.timerReq(name, dur, tags)
	if err != nil {
		// Stat reporter can't return errs, just print it.
		a.logger.Printf("Error: %v\n", err)
		return
	}

	resp, err := a.cli.Do(req)
	if err != nil {
		a.logger.Printf("http.Post: %v\n", err)
		return
	}
	if resp.StatusCode != 200 {
		a.logger.Printf("http.Post returned status: %s\n", resp.Status)
	}

}

func (a *remoteAnalytics) Flush(timeout time.Duration) {
	ch := make(chan bool)
	go func() {
		a.wg.Wait()
		close(ch)
	}()

	select {
	case <-time.After(timeout):
	case <-ch:
	}
}

func (a *remoteAnalytics) timerReq(name string, dur time.Duration, tags map[string]string) (*http.Request, error) {
	reqBody := a.baseReqBody(name, tags)
	reqBody[keyDuration] = dur
	return a.makeReq(reqBody)
}

type MemoryAnalytics struct {
	Counts []CountEvent
	Timers []TimeEvent
}

type CountEvent struct {
	name string
	tags map[string]string
	n    int
}

type TimeEvent struct {
	name string
	tags map[string]string
	dur  time.Duration
}

func NewMemoryAnalytics() *MemoryAnalytics {
	return &MemoryAnalytics{}
}

func (a *MemoryAnalytics) Count(name string, tags map[string]string, n int) {
	a.Counts = append(a.Counts, CountEvent{name: name, tags: tags, n: n})
}

func (a *MemoryAnalytics) Incr(name string, tags map[string]string) {
	a.Count(name, tags, 1)
}

func (a *MemoryAnalytics) Timer(name string, dur time.Duration, tags map[string]string) {
	a.Timers = append(a.Timers, TimeEvent{name: name, dur: dur, tags: tags})
}

func (a *MemoryAnalytics) Flush(timeout time.Duration) {}

var _ Analytics = &remoteAnalytics{}
var _ Analytics = &MemoryAnalytics{}
