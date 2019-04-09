package server_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/hud/server"
	"github.com/windmilleng/tilt/internal/store"
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

	var jsonStr = []byte(`[{"Verb": "incr", "Name": "foo", "Tags": {}}]`)
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

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusInternalServerError)
	}
}

func TestHandleAnalyticsErrorsIfNotIncr(t *testing.T) {
	f := newTestFixture(t)

	var jsonStr = []byte(`[{"Verb": "count", "Name": "foo", "Tags": {}}]`)
	req, err := http.NewRequest(http.MethodPost, "/api/analytics", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.s.HandleAnalytics)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusInternalServerError)
	}
}

type serverFixture struct {
	t *testing.T
	s server.HeadsUpServer
	a *fakeAnalytics
}

func newTestFixture(t *testing.T) *serverFixture {
	st := store.NewStore(engine.UpperReducer, store.LogActionsFlag(false))
	a := &fakeAnalytics{
		increments: map[string]int{},
	}
	s := server.ProvideHeadsUpServer(st, server.NewFakeAssetServer(), a)

	return &serverFixture{
		t: t,
		s: s,
		a: a,
	}
}

func (f *serverFixture) assertIncrement(name string, count int) {
	contains := assert.Contains(f.t, f.a.increments, name)
	if contains {
		assert.Equal(f.t, count, f.a.increments[name])
	}
}

type fakeAnalytics struct {
	increments map[string]int
}

func (a *fakeAnalytics) Count(name string, tags map[string]string, n int) {}
func (a *fakeAnalytics) Incr(name string, tags map[string]string) {
	a.increments[name]++
}
func (a *fakeAnalytics) Timer(name string, dur time.Duration, tags map[string]string) {}
func (a *fakeAnalytics) Flush(timeout time.Duration)                                  {}
