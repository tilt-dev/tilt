package server_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/wmclient/pkg/analytics"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/assets"
	"github.com/windmilleng/tilt/internal/hud/server"
	"github.com/windmilleng/tilt/internal/sail/client"
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
	handler := http.HandlerFunc(f.serv.HandleAnalytics)

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
	handler := http.HandlerFunc(f.serv.HandleAnalytics)

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
	handler := http.HandlerFunc(f.serv.HandleAnalytics)

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
	handler := http.HandlerFunc(f.serv.HandleAnalytics)

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
	handler := http.HandlerFunc(f.serv.HandleAnalytics)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestHandleAnalyticsOptIn(t *testing.T) {
	f := newTestFixture(t)

	var jsonStr = []byte(`{"opt": "opt-in"}`)
	req, err := http.NewRequest(http.MethodPost, "/api/analytics_opt", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.serv.HandleAnalyticsOpt)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	action := store.WaitForAction(t, reflect.TypeOf(store.AnalyticsOptAction{}), f.getActions)
	assert.Equal(t, store.AnalyticsOptAction{Opt: analytics.OptIn}, action)
}

func TestHandleAnalyticsOptNonPost(t *testing.T) {
	f := newTestFixture(t)

	req, err := http.NewRequest(http.MethodGet, "/api/analytics_opt", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.serv.HandleAnalyticsOpt)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestHandleAnalyticsOptMalformedPayload(t *testing.T) {
	f := newTestFixture(t)

	var jsonStr = []byte(`{"opt":`)
	req, err := http.NewRequest(http.MethodPost, "/api/analytics_opt", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.serv.HandleAnalyticsOpt)

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
	handler := http.HandlerFunc(f.serv.HandleSail)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	assert.Equal(f.t, 1, f.sailCli.ConnectCalls)
}

func TestHandleTriggerReturnsError(t *testing.T) {
	f := newTestFixture(t)

	var jsonStr = []byte(`{"manifest_name":"foo"`)
	req, err := http.NewRequest(http.MethodPost, "/api/trigger", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.serv.HandleTrigger)

	handler.ServeHTTP(rr, req)

	// Expect maybeSendToTrigerQueue to fail: make sure we reply to the HTTP request
	// with an error when this happens
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestHandleTriggerNonPost(t *testing.T) {
	f := newTestFixture(t)

	req, err := http.NewRequest(http.MethodGet, "/api/trigger", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.serv.HandleTrigger)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestHandleTriggerMalformedPayload(t *testing.T) {
	f := newTestFixture(t)

	var jsonStr = []byte(`{"manifest_name":`)
	req, err := http.NewRequest(http.MethodPost, "/api/trigger", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.serv.HandleTrigger)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestMaybeSendToTriggerQueue(t *testing.T) {
	f := newTestFixture(t)

	mt := store.ManifestTarget{
		Manifest: model.Manifest{
			Name:        "foobar",
			TriggerMode: model.TriggerModeManual,
		},
	}
	state := f.st.LockMutableStateForTesting()
	state.UpsertManifestTarget(&mt)
	f.st.UnlockMutableState()

	err := server.MaybeSendToTriggerQueue(f.st, "foobar")
	if err != nil {
		t.Fatal(err)
	}

	store.WaitForAction(t, reflect.TypeOf(server.AppendToTriggerQueueAction{}), f.getActions)
}

func TestMaybeSendToTriggerQueue_noManifestWithName(t *testing.T) {
	f := newTestFixture(t)

	err := server.MaybeSendToTriggerQueue(f.st, "foobar")

	assert.EqualError(t, err, "no manifest found with name 'foobar'")
	store.AssertNoActionOfType(t, reflect.TypeOf(server.AppendToTriggerQueueAction{}), f.getActions)
}

func TestMaybeSendToTriggerQueue_notManualManifest(t *testing.T) {
	f := newTestFixture(t)

	mt := store.ManifestTarget{
		Manifest: model.Manifest{
			Name:        "foobar",
			TriggerMode: model.TriggerModeAuto,
		},
	}
	state := f.st.LockMutableStateForTesting()
	state.UpsertManifestTarget(&mt)
	f.st.UnlockMutableState()

	err := server.MaybeSendToTriggerQueue(f.st, "foobar")

	assert.EqualError(t, err, "can only trigger updates for manifests of TriggerModeManual")
	store.AssertNoActionOfType(t, reflect.TypeOf(server.AppendToTriggerQueueAction{}), f.getActions)
}

type serverFixture struct {
	t          *testing.T
	serv       *server.HeadsUpServer
	a          *analytics.MemoryAnalytics
	sailCli    *client.FakeSailClient
	st         *store.Store
	getActions func() []store.Action
}

func newTestFixture(t *testing.T) *serverFixture {
	st, getActions := store.NewStoreForTesting()
	go st.Loop(context.Background())
	a := analytics.NewMemoryAnalytics()
	a, ta := tiltanalytics.NewMemoryTiltAnalyticsForTest(tiltanalytics.NullOpter{})
	sailCli := client.NewFakeSailClient()
	serv := server.ProvideHeadsUpServer(st, assets.NewFakeServer(), ta, sailCli)

	return &serverFixture{
		t:          t,
		serv:       serv,
		a:          a,
		sailCli:    sailCli,
		st:         st,
		getActions: getActions,
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
