package server_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/assets"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/hud/server"
	"github.com/windmilleng/tilt/internal/sail/client"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

func TestHandleAnalyticsEmptyRequest(t *testing.T) {
	f := newTestFixture(t)

	var jsonStr = []byte(`[]`)
	req, err := http.NewRequest(http.MethodPost, "/api/analytics", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.s.HandleAnalytics)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestHandleAnalyticsRecordsIncr(t *testing.T) {
	f := newTestFixture(t)

	var jsonStr = []byte(`[{"verb": "incr", "name": "foo", "tags": {}}]`)
	req, err := http.NewRequest(http.MethodPost, "/api/analytics", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.s.HandleAnalytics)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	f.assertIncrement("foo", 1)
}

func TestHandleAnalyticsNonPost(t *testing.T) {
	f := newTestFixture(t)

	req, err := http.NewRequest(http.MethodGet, "/api/analytics", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.s.HandleAnalytics)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestHandleAnalyticsMalformedPayload(t *testing.T) {
	f := newTestFixture(t)

	var jsonStr = []byte(`[{"Verb": ]`)
	req, err := http.NewRequest(http.MethodPost, "/api/analytics", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.s.HandleAnalytics)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestHandleAnalyticsErrorsIfNotIncr(t *testing.T) {
	f := newTestFixture(t)

	var jsonStr = []byte(`[{"verb": "count", "name": "foo", "tags": {}}]`)
	req, err := http.NewRequest(http.MethodPost, "/api/analytics", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.s.HandleAnalytics)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestHandleSail(t *testing.T) {
	f := newTestFixture(t)

	req, err := http.NewRequest(http.MethodPost, "/api/sail", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.s.HandleSail)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	assert.Equal(f.t, 1, f.sailCli.ConnectCalls)
}

func TestResetRestarts(t *testing.T) {
	f := newTestFixture(t)

	req, err := http.NewRequest(http.MethodGet, "/api/control/reset_restarts", nil)
	if err != nil {
		t.Fatal(err)
	}
	q := req.URL.Query()
	q.Add("name", "foo")
	req.URL.RawQuery = q.Encode()

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.s.HandleResetRestarts)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestResetRestartsNoParam(t *testing.T) {
	f := newTestFixture(t)

	req, err := http.NewRequest(http.MethodGet, "/api/control/reset_restarts", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.s.HandleResetRestarts)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}

type serverFixture struct {
	t       *testing.T
	s       *server.HeadsUpServer
	a       *analytics.MemoryAnalytics
	sailCli *client.FakeSailClient
}

func newTestFixture(t *testing.T) *serverFixture {
	st := store.NewStore(engine.UpperReducer, store.LogActionsFlag(false))
	a := analytics.NewMemoryAnalytics()
	sailCli := client.NewFakeSailClient()
	s := server.ProvideHeadsUpServer(st, assets.NewFakeServer(), a, sailCli, newTestOpter(t))

	return &serverFixture{
		t:       t,
		s:       s,
		a:       a,
		sailCli: sailCli,
	}
}

func (f *serverFixture) assertIncrement(name string, count int) {
	runningCount := 0
	for _, c := range f.a.Counts {
		if c.Name == name {
			runningCount += c.N
		}
	}

	assert.Equalf(f.t, count, runningCount, "Expected the total count to be %d, got %d", count, runningCount)
}

type testOpter struct {
	t        *testing.T
	nextErr  error
	callStrs []string
}

var _ server.AnalyticsOpter = &testOpter{}

func newTestOpter(t *testing.T) *testOpter { return &testOpter{t: t} }

func (o *testOpter) SetOptStr(s string) error {
	o.callStrs = append(o.callStrs, s)

	if o.nextErr != nil {
		err := o.nextErr
		o.nextErr = nil
		return err
	}

	return nil
}
