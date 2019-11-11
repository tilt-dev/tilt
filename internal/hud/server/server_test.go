package server_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/windmilleng/wmclient/pkg/analytics"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/cloud"
	"github.com/windmilleng/tilt/internal/cloud/cloudurl"
	"github.com/windmilleng/tilt/internal/hud/server"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/assets"
	"github.com/windmilleng/tilt/pkg/model"
	proto_webview "github.com/windmilleng/tilt/pkg/webview"
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

	err := f.ta.SetUserOpt(analytics.OptDefault)
	if err != nil {
		t.Fatal(err)
	}

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

	action := store.WaitForAction(t, reflect.TypeOf(store.AnalyticsUserOptAction{}), f.getActions)
	assert.Equal(t, store.AnalyticsUserOptAction{Opt: analytics.OptIn}, action)

	f.a.Flush(time.Millisecond)

	assert.Equal(t, []analytics.CountEvent{{
		Name: "analytics.opt.in",
		Tags: map[string]string{"version": "v0.0.0"},
		N:    1,
	}}, f.a.Counts)
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

func TestHandleTriggerReturnsError(t *testing.T) {
	f := newTestFixture(t)

	var jsonStr = []byte(`{"manifest_names":["foo"]}`)
	req, err := http.NewRequest(http.MethodPost, "/api/trigger", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.serv.HandleTrigger)

	handler.ServeHTTP(rr, req)

	// Expect SendToTriggerQueue to fail: make sure we reply to the HTTP request
	// with an error when this happens
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
	assert.Contains(t, rr.Body.String(), "no manifest found with name")
}

func TestHandleTriggerTooManyManifestNames(t *testing.T) {
	f := newTestFixture(t)

	var jsonStr = []byte(`{"manifest_names":["foo", "bar"]}`)
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
	assert.Contains(t, rr.Body.String(), "currently supports exactly one manifest name, got 2")
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
	assert.Contains(t, rr.Body.String(), "must be POST request")
}

func TestHandleTriggerMalformedPayload(t *testing.T) {
	f := newTestFixture(t)

	var jsonStr = []byte(`{"manifest_names":`)
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
	assert.Contains(t, rr.Body.String(), "error parsing JSON")
}

func TestSendToTriggerQueue_manualManifest(t *testing.T) {
	f := newTestFixture(t)

	mt := store.ManifestTarget{
		Manifest: model.Manifest{
			Name:        "foobar",
			TriggerMode: model.TriggerModeManualAfterInitial,
		},
	}
	state := f.st.LockMutableStateForTesting()
	state.UpsertManifestTarget(&mt)
	f.st.UnlockMutableState()

	err := server.SendToTriggerQueue(f.st, "foobar")
	if err != nil {
		t.Fatal(err)
	}

	a := store.WaitForAction(t, reflect.TypeOf(server.AppendToTriggerQueueAction{}), f.getActions)
	action, ok := a.(server.AppendToTriggerQueueAction)
	if !ok {
		t.Fatalf("Action was not of type 'AppendToTriggerQueueAction': %+v", action)
	}
	assert.Equal(t, "foobar", action.Name.String())
}

func TestSendToTriggerQueue_automaticManifest(t *testing.T) {
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

	err := server.SendToTriggerQueue(f.st, "foobar")
	if err != nil {
		t.Fatal(err)
	}

	a := store.WaitForAction(t, reflect.TypeOf(server.AppendToTriggerQueueAction{}), f.getActions)
	action, ok := a.(server.AppendToTriggerQueueAction)
	if !ok {
		t.Fatalf("Action was not of type 'AppendToTriggerQueueAction': %+v", action)
	}
	assert.Equal(t, "foobar", action.Name.String())
}

func TestSendToTriggerQueue_noManifestWithName(t *testing.T) {
	f := newTestFixture(t)

	err := server.SendToTriggerQueue(f.st, "foobar")

	assert.EqualError(t, err, "no manifest found with name 'foobar'")
	store.AssertNoActionOfType(t, reflect.TypeOf(server.AppendToTriggerQueueAction{}), f.getActions)
}

func TestHandleNewSnapshot(t *testing.T) {
	f := newTestFixture(t)

	sp := filepath.Join("..", "webview", "testdata", "snapshot.json")
	snap, err := ioutil.ReadFile(sp)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, "/api/snapshot/new", bytes.NewBuffer(snap))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.serv.HandleNewSnapshot)

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code,
		"handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	require.Contains(t, rr.Body.String(), "https://nonexistent.example.com/snapshot/aaaaa")

	lastReq := f.snapshotHTTP.lastReq
	if assert.NotNil(t, lastReq) {
		var snapshot proto_webview.Snapshot
		jspb := &runtime.JSONPb{OrigName: false, EmitDefaults: true}
		decoder := jspb.NewDecoder(lastReq.Body)
		decoder.Decode(&snapshot)
		assert.Equal(t, "0.10.13", snapshot.View.RunningTiltBuild.Version)
		assert.Equal(t, "43", snapshot.SnapshotHighlight.BeginningLogID)
	}
}

type serverFixture struct {
	t            *testing.T
	serv         *server.HeadsUpServer
	a            *analytics.MemoryAnalytics
	ta           *tiltanalytics.TiltAnalytics
	st           *store.Store
	getActions   func() []store.Action
	snapshotHTTP *fakeHTTPClient
}

func newTestFixture(t *testing.T) *serverFixture {
	st, getActions := store.NewStoreForTesting()
	go st.Loop(context.Background())
	a := analytics.NewMemoryAnalytics()
	opter := tiltanalytics.NewFakeOpter(analytics.OptIn)
	a, ta := tiltanalytics.NewMemoryTiltAnalyticsForTest(opter)
	snapshotHTTP := &fakeHTTPClient{}
	addr := cloudurl.Address("nonexistent.example.com")
	uploader := cloud.NewSnapshotUploader(snapshotHTTP, addr)
	serv, err := server.ProvideHeadsUpServer(context.Background(), st, assets.NewFakeServer(), ta, uploader)
	if err != nil {
		t.Fatal(err)
	}

	return &serverFixture{
		t:            t,
		serv:         serv,
		a:            a,
		ta:           ta,
		st:           st,
		getActions:   getActions,
		snapshotHTTP: snapshotHTTP,
	}
}

type fakeHTTPClient struct {
	lastReq *http.Request
}

func (f *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	f.lastReq = req

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(bytes.NewReader([]byte(`{"ID":"aaaaa"}`))),
	}, nil
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
