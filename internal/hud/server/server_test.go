package server_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	grpcRuntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	tiltanalytics "github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/cloud"
	"github.com/tilt-dev/tilt/internal/cloud/cloudurl"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/hud/view"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/model"
	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
	"github.com/tilt-dev/wmclient/pkg/analytics"
)

func TestHandleAnalyticsEmptyRequest(t *testing.T) {
	f := newTestFixture(t)

	status, _ := f.makeReq("/api/analytics", f.serv.HandleAnalytics, http.MethodPost, "[]")
	require.Equal(t, http.StatusOK, status, "handler returned wrong status code")
}

func TestHandleAnalyticsRecordsIncr(t *testing.T) {
	f := newTestFixture(t)

	payload := `[{"verb": "incr", "name": "foo", "tags": {}}]`

	status, _ := f.makeReq("/api/analytics", f.serv.HandleAnalytics, http.MethodPost, payload)
	require.Equal(t, http.StatusOK, status, "handler returned wrong status code")

	f.assertIncrement("foo", 1)
}

func TestHandleAnalyticsNonPost(t *testing.T) {
	f := newTestFixture(t)

	status, respBody := f.makeReq("/api/analytics", f.serv.HandleAnalytics, http.MethodGet, "")

	require.Equal(t, http.StatusBadRequest, status, "handler returned wrong status code")
	require.Contains(t, respBody, "must be POST request")
}

func TestHandleAnalyticsMalformedPayload(t *testing.T) {
	f := newTestFixture(t)

	payload := `[{"Verb": ]`
	status, respBody := f.makeReq("/api/analytics", f.serv.HandleAnalytics, http.MethodPost, payload)

	require.Equal(t, http.StatusBadRequest, status, "handler returned wrong status code")
	require.Contains(t, respBody, "error parsing JSON")
}

func TestHandleAnalyticsErrorsIfNotIncr(t *testing.T) {
	f := newTestFixture(t)

	payload := `[{"verb": "count", "name": "foo", "tags": {}}]`
	status, respBody := f.makeReq("/api/analytics", f.serv.HandleAnalytics, http.MethodPost, payload)

	require.Equal(t, http.StatusBadRequest, status, "handler returned wrong status code")
	require.Contains(t, respBody, "only incr verbs are supported")
}

func TestHandleAnalyticsOptIn(t *testing.T) {
	f := newTestFixture(t)

	err := f.ta.SetUserOpt(analytics.OptDefault)
	if err != nil {
		t.Fatal(err)
	}

	payload := `{"opt": "opt-in"}`
	status, _ := f.makeReq("/api/analytics", f.serv.HandleAnalyticsOpt, http.MethodPost, payload)

	require.Equal(t, http.StatusOK, status, "handler returned wrong status code")

	action := store.WaitForAction(t, reflect.TypeOf(store.AnalyticsUserOptAction{}), f.getActions)
	assert.Equal(t, store.AnalyticsUserOptAction{Opt: analytics.OptIn}, action)

	f.a.Flush(time.Millisecond)

	assert.Equal(t, []analytics.CountEvent{{
		Name: "analytics.opt.in",
		N:    1,
	}}, f.a.Counts)
}

func TestHandleAnalyticsOptNonPost(t *testing.T) {
	f := newTestFixture(t)
	status, respBody := f.makeReq("/api/analytics", f.serv.HandleAnalyticsOpt, http.MethodGet, "")

	require.Equal(t, http.StatusBadRequest, status, "handler returned wrong status code")
	require.Contains(t, respBody, "must be POST request")
}

func TestHandleAnalyticsOptMalformedPayload(t *testing.T) {
	f := newTestFixture(t)

	payload := `{"opt":`
	status, respBody := f.makeReq("/api/analytics", f.serv.HandleAnalyticsOpt, http.MethodPost, payload)

	require.Equal(t, http.StatusBadRequest, status, "handler returned wrong status code")
	require.Contains(t, respBody, "error parsing JSON")
}

func TestHandleTriggerNoManifestWithName(t *testing.T) {
	f := newTestFixture(t)

	payload := `{"manifest_names":["foo"]}`
	status, respBody := f.makeReq("/api/trigger", f.serv.HandleTrigger, http.MethodPost, payload)

	// Expect SendToTriggerQueue to fail: make sure we reply to the HTTP request
	// with an error when this happens
	require.Equal(t, http.StatusOK, status, "handler returned wrong status code")
	require.Equal(t, respBody, "resource \"foo\" does not exist")
}

func TestHandleTriggerTooManyManifestNames(t *testing.T) {
	f := newTestFixture(t)

	payload := `{"manifest_names":["foo", "bar"]}`
	status, respBody := f.makeReq("/api/trigger", f.serv.HandleTrigger, http.MethodPost, payload)

	require.Equal(t, http.StatusBadRequest, status, "handler returned wrong status code")
	require.Contains(t, respBody, "currently supports exactly one manifest name, got 2")
}

func TestHandleTriggerNonPost(t *testing.T) {
	f := newTestFixture(t)

	status, respBody := f.makeReq("/api/trigger", f.serv.HandleTrigger, http.MethodGet, "")

	require.Equal(t, http.StatusBadRequest, status, "handler returned wrong status code")
	require.Contains(t, respBody, "must be POST request")
}

func TestHandleTriggerMalformedPayload(t *testing.T) {
	f := newTestFixture(t)

	payload := `{"manifest_names":`
	status, respBody := f.makeReq("/api/trigger", f.serv.HandleTrigger, http.MethodPost, payload)

	require.Equal(t, http.StatusBadRequest, status, "handler returned wrong status code")
	require.Contains(t, respBody, "error parsing JSON")
}

func TestHandleTriggerTiltfileOK(t *testing.T) {
	f := newTestFixture(t)

	payload := fmt.Sprintf(`{"manifest_names":["%s"]}`, model.MainTiltfileManifestName)
	status, _ := f.makeReq("/api/trigger", f.serv.HandleTrigger, http.MethodPost, payload)

	require.Equal(t, http.StatusOK, status, "handler returned wrong status code")
}

func TestHandleTriggerResourceDisabled(t *testing.T) {
	f := newTestFixture(t)

	f.withDummyManifests("foo")
	state := f.st.LockMutableStateForTesting()
	state.ManifestTargets["foo"].State.DisableState = v1alpha1.DisableStateDisabled
	f.st.UnlockMutableState()
	payload := `{"manifest_names": ["foo"]}`
	status, body := f.makeReq("/api/trigger", f.serv.HandleTrigger, http.MethodPost, payload)

	require.Equal(t, http.StatusOK, status, "handler returned wrong status code")
	require.Equal(t, "resource \"foo\" is currently disabled", body)
}

func TestSendToTriggerQueue_manualManifest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO(nick): fix this")
	}
	f := newTestFixture(t)

	mt := store.ManifestTarget{
		Manifest: model.Manifest{
			Name:        "foobar",
			TriggerMode: model.TriggerModeManualWithAutoInit,
		},
	}
	state := f.st.LockMutableStateForTesting()
	state.UpsertManifestTarget(&mt)
	f.st.UnlockMutableState()

	err := server.SendToTriggerQueue(f.st, "foobar", model.BuildReasonFlagTriggerWeb)
	if err != nil {
		t.Fatal(err)
	}

	a := store.WaitForAction(t, reflect.TypeOf(server.AppendToTriggerQueueAction{}), f.getActions)
	action, ok := a.(server.AppendToTriggerQueueAction)
	if !ok {
		t.Fatalf("Action was not of type 'AppendToTriggerQueueAction': %+v", action)
	}
	assert.Equal(t, "foobar", action.Name.String())
	assert.Equal(t, model.BuildReasonFlagTriggerWeb, action.Reason)
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

	err := server.SendToTriggerQueue(f.st, "foobar", model.BuildReasonFlagTriggerWeb)
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

func TestSendToTriggerQueue_Tiltfile(t *testing.T) {
	f := newTestFixture(t)

	err := server.SendToTriggerQueue(f.st, model.MainTiltfileManifestName.String(), model.BuildReasonFlagTriggerWeb)
	if err != nil {
		t.Fatal(err)
	}

	a := store.WaitForAction(t, reflect.TypeOf(server.AppendToTriggerQueueAction{}), f.getActions)
	action, ok := a.(server.AppendToTriggerQueueAction)
	if !ok {
		t.Fatalf("Action was not of type 'AppendToTriggreQueueAction': %+v", action)
	}

	expected := server.AppendToTriggerQueueAction{
		Name:   model.MainTiltfileManifestName,
		Reason: model.BuildReasonFlagTriggerWeb,
	}
	assert.Equal(t, expected, action)
}

func TestSendToTriggerQueue_noManifestWithName(t *testing.T) {
	f := newTestFixture(t)

	err := server.SendToTriggerQueue(f.st, "foobar", model.BuildReasonFlagTriggerWeb)

	assert.EqualError(t, err, "resource \"foobar\" does not exist")
	store.AssertNoActionOfType(t, reflect.TypeOf(server.AppendToTriggerQueueAction{}), f.getActions)
}

func TestHandleOverrideTriggerModeReturnsErrorForBadManifest(t *testing.T) {
	f := newTestFixture(t).withDummyManifests("foo", "baz")

	payload := `{"manifest_names":["foo", "bar", "baz"]}`
	status, respBody := f.makeReq("/api/override/trigger_mode", f.serv.HandleOverrideTriggerMode, http.MethodPost, payload)

	require.Equal(t, http.StatusBadRequest, status, "handler returned wrong status code")
	require.Contains(t, respBody, "no manifest found with name 'bar'")
	store.AssertNoActionOfType(t, reflect.TypeOf(server.OverrideTriggerModeAction{}), f.getActions)
}

func TestHandleOverrideTriggerModeNonPost(t *testing.T) {
	f := newTestFixture(t)

	status, respBody := f.makeReq("/api/override/trigger_mode", f.serv.HandleOverrideTriggerMode, http.MethodGet, "")

	require.Equal(t, http.StatusBadRequest, status, "handler returned wrong status code")
	require.Contains(t, respBody, "must be POST request")
	store.AssertNoActionOfType(t, reflect.TypeOf(server.OverrideTriggerModeAction{}), f.getActions)
}

func TestHandleOverrideTriggerModeMalformedPayload(t *testing.T) {
	f := newTestFixture(t)

	payload := `{"manifest_names":`
	status, respBody := f.makeReq("/api/override/trigger_mode", f.serv.HandleOverrideTriggerMode, http.MethodPost, payload)

	require.Equal(t, http.StatusBadRequest, status, "handler returned wrong status code")
	require.Contains(t, respBody, "error parsing JSON")
	store.AssertNoActionOfType(t, reflect.TypeOf(server.OverrideTriggerModeAction{}), f.getActions)
}

func TestHandleOverrideTriggerModeInvalidTriggerMode(t *testing.T) {
	f := newTestFixture(t).withDummyManifests("foo")

	payload := `{"manifest_names":["foo"], "trigger_mode": 12345}`
	status, respBody := f.makeReq("/api/override/trigger_mode", f.serv.HandleOverrideTriggerMode, http.MethodPost, payload)

	require.Equal(t, http.StatusBadRequest, status, "handler returned wrong status code")
	require.Contains(t, respBody, "invalid trigger mode: 12345")
	store.AssertNoActionOfType(t, reflect.TypeOf(server.OverrideTriggerModeAction{}), f.getActions)
}

func TestHandleOverrideTriggerModeDispatchesEvent(t *testing.T) {
	f := newTestFixture(t).withDummyManifests("foo", "bar")

	payload := fmt.Sprintf(`{"manifest_names":["foo", "bar"], "trigger_mode": %d}`,
		model.TriggerModeManualWithAutoInit)
	status, _ := f.makeReq("/api/override/trigger_mode", f.serv.HandleOverrideTriggerMode, http.MethodPost, payload)

	require.Equal(t, http.StatusOK, status, "handler returned wrong status code")

	a := store.WaitForAction(t, reflect.TypeOf(server.OverrideTriggerModeAction{}), f.getActions)
	action, ok := a.(server.OverrideTriggerModeAction)
	if !ok {
		t.Fatalf("Action was not of type 'OverrideTriggerModeAction': %+v", action)
	}

	expected := server.OverrideTriggerModeAction{
		ManifestNames: []model.ManifestName{"foo", "bar"},
		TriggerMode:   model.TriggerModeManualWithAutoInit,
	}
	assert.Equal(t, expected, action)
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
		jspb := &grpcRuntime.JSONPb{}
		decoder := jspb.NewDecoder(lastReq.Body)
		err := decoder.Decode(&snapshot)
		require.NoError(t, err)
		assert.Equal(t, "0.10.13", snapshot.View.RunningTiltBuild.Version)
		assert.Equal(t, "43", snapshot.SnapshotHighlight.BeginningLogID)
	}
}

func TestSetTiltfileArgs(t *testing.T) {
	f := newTestFixture(t)

	json := `["--foo", "bar", "as df"]`
	req, err := http.NewRequest("POST", "/api/set_tiltfile_args", strings.NewReader(json))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(f.serv.HandleSetTiltfileArgs)

	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	require.Eventuallyf(t, func() bool {
		var tf v1alpha1.Tiltfile
		err := f.ctrlClient.Get(f.ctx, types.NamespacedName{Name: view.TiltfileResourceName}, &tf)
		if err != nil {
			return false
		}
		return sliceutils.StringSliceEquals(tf.Spec.Args, []string{"--foo", "bar", "as df"})
	},
		time.Second, time.Millisecond, "args didn't show up in Tiltfile API object",
	)
}

type serverFixture struct {
	t            *testing.T
	ctx          context.Context
	serv         *server.HeadsUpServer
	a            *analytics.MemoryAnalytics
	ta           *tiltanalytics.TiltAnalytics
	st           *store.Store
	ctrlClient   ctrlclient.Client
	getActions   func() []store.Action
	snapshotHTTP *fakeHTTPClient
}

func newTestFixture(t *testing.T) *serverFixture {
	st, getActions := store.NewStoreWithFakeReducer()
	go func() {
		err := st.Loop(context.Background())
		testutils.FailOnNonCanceledErr(t, err, "store.Loop failed")
	}()
	opter := tiltanalytics.NewFakeOpter(analytics.OptIn)
	a, ta := tiltanalytics.NewMemoryTiltAnalyticsForTest(opter)
	snapshotHTTP := &fakeHTTPClient{}
	addr := cloudurl.Address("nonexistent.example.com")
	uploader := cloud.NewSnapshotUploader(snapshotHTTP, addr)
	wsl := server.NewWebsocketList()
	ctrlClient := fake.NewFakeTiltClient()
	_ = ctrlClient.Create(context.Background(), &v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{Name: model.MainTiltfileManifestName.String()},
	})

	ctx := context.Background()

	serv, err := server.ProvideHeadsUpServer(ctx, st, assets.NewFakeServer(), ta, uploader, wsl, ctrlClient)
	if err != nil {
		t.Fatal(err)
	}

	return &serverFixture{
		t:            t,
		ctx:          ctx,
		serv:         serv,
		a:            a,
		ta:           ta,
		st:           st,
		ctrlClient:   ctrlClient,
		getActions:   getActions,
		snapshotHTTP: snapshotHTTP,
	}
}

func (f *serverFixture) makeReq(endpoint string, handler http.HandlerFunc,
	method, body string) (statusCode int, respBody string) {
	var reader io.Reader
	if method == http.MethodPost {
		reader = bytes.NewBuffer([]byte(body))
	}
	req, err := http.NewRequest(method, endpoint, reader)
	if err != nil {
		f.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	return rr.Code, rr.Body.String()
}

func (f *serverFixture) withDummyManifests(mNames ...string) *serverFixture {
	state := f.st.LockMutableStateForTesting()
	for _, mName := range mNames {
		m := model.Manifest{Name: model.ManifestName(mName)}
		mt := store.NewManifestTarget(m)
		state.UpsertManifestTarget(mt)
	}
	defer f.st.UnlockMutableState()
	return f
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
