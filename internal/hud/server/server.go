package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
	"unsafe"

	"github.com/gorilla/mux"
	_ "github.com/gorilla/websocket"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
	"github.com/pkg/errors"
	"github.com/windmilleng/wmclient/pkg/analytics"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/cloud"
	"github.com/windmilleng/tilt/internal/hud/webview"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/assets"
	"github.com/windmilleng/tilt/pkg/model"
	proto_webview "github.com/windmilleng/tilt/pkg/webview"
)

const httpTimeOut = 5 * time.Second
const TiltTokenCookieName = "Tilt-Token"

type analyticsPayload struct {
	Verb string            `json:"verb"`
	Name string            `json:"name"`
	Tags map[string]string `json:"tags"`
}

type analyticsOptPayload struct {
	Opt string `json:"opt"`
}

type triggerPayload struct {
	ManifestNames []string `json:"manifest_names"`
}

type actionPayload struct {
	Type            string             `json:"type"`
	ManifestName    model.ManifestName `json:"manifest_name"`
	PodID           k8s.PodID          `json:"pod_id"`
	VisibleRestarts int                `json:"visible_restarts"`
}

type HeadsUpServer struct {
	store             *store.Store
	router            *mux.Router
	a                 *tiltanalytics.TiltAnalytics
	uploader          cloud.SnapshotUploader
	numWebsocketConns int32
}

func ProvideHeadsUpServer(
	ctx context.Context,
	store *store.Store,
	assetServer assets.Server,
	analytics *tiltanalytics.TiltAnalytics,
	uploader cloud.SnapshotUploader) (*HeadsUpServer, error) {
	r := mux.NewRouter().UseEncodedPath()
	s := &HeadsUpServer{
		store:    store,
		router:   r,
		a:        analytics,
		uploader: uploader,
	}

	r.HandleFunc("/api/view", s.ViewJSON)
	r.HandleFunc("/api/analytics", s.HandleAnalytics)
	r.HandleFunc("/api/analytics_opt", s.HandleAnalyticsOpt)
	r.HandleFunc("/api/trigger", s.HandleTrigger)
	r.HandleFunc("/api/action", s.DispatchAction).Methods("POST")
	r.HandleFunc("/api/snapshot/new", s.HandleNewSnapshot).Methods("POST")
	// this endpoint is only used for testing snapshots in development
	r.HandleFunc("/api/snapshot/{snapshot_id}", s.SnapshotJSON)
	r.HandleFunc("/ws/view", s.ViewWebsocket)
	r.HandleFunc("/api/user_started_tilt_cloud_registration", s.userStartedTiltCloudRegistration)

	r.PathPrefix("/").Handler(s.cookieWrapper(assetServer))

	return s, nil
}

type funcHandler struct {
	f func(w http.ResponseWriter, r *http.Request)
}

func (fh funcHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fh.f(w, r)
}

func (s *HeadsUpServer) cookieWrapper(handler http.Handler) http.Handler {
	return funcHandler{f: func(w http.ResponseWriter, r *http.Request) {
		state := s.store.RLockState()
		http.SetCookie(w, &http.Cookie{Name: TiltTokenCookieName, Value: string(state.Token), Path: "/"})
		s.store.RUnlockState()
		handler.ServeHTTP(w, r)
	}}
}

func (s *HeadsUpServer) Router() http.Handler {
	return s.router
}

func (s *HeadsUpServer) ViewJSON(w http.ResponseWriter, req *http.Request) {
	state := s.store.RLockState()
	view := webview.StateToProtoView(state)
	s.store.RUnlockState()

	jsEncoder := &runtime.JSONPb{OrigName: false, EmitDefaults: true}

	w.Header().Set("Content-Type", "application/json")
	err := jsEncoder.NewEncoder(w).Encode(view)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error rendering view payload: %v", err), http.StatusInternalServerError)
	}
}

func (s *HeadsUpServer) SnapshotJSON(w http.ResponseWriter, req *http.Request) {
	state := s.store.RLockState()
	view := webview.StateToWebView(state)
	s.store.RUnlockState()

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(cloud.Snapshot{
		View: view,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Error rendering view payload: %v", err), http.StatusInternalServerError)
	}
}

func (s *HeadsUpServer) HandleAnalyticsOpt(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "must be POST request", http.StatusBadRequest)
		return
	}

	var payload analyticsOptPayload

	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&payload)
	if err != nil {
		http.Error(w, fmt.Sprintf("error parsing JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	opt, err := analytics.ParseOpt(payload.Opt)
	if err != nil {
		http.Error(w, fmt.Sprintf("error parsing opt '%s': %v", payload.Opt, err), http.StatusBadRequest)
	}

	// only logging on opt-in, because, well, opting out means the user just told us not to report data on them!
	if opt == analytics.OptIn {
		s.a.IncrIfUnopted("analytics.opt.in")
	}

	s.store.Dispatch(store.AnalyticsOptAction{Opt: opt})
}

func (s *HeadsUpServer) HandleAnalytics(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "must be POST request", http.StatusBadRequest)
		return
	}

	var payloads []analyticsPayload

	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&payloads)
	if err != nil {
		http.Error(w, fmt.Sprintf("error parsing JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	for _, p := range payloads {
		if p.Verb != "incr" {
			http.Error(w, "error parsing payloads: only incr verbs are supported", http.StatusBadRequest)
			return
		}

		s.a.Incr(p.Name, p.Tags)
	}
}

func (s *HeadsUpServer) DispatchAction(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "must be POST request", http.StatusBadRequest)
		return
	}

	var payload actionPayload
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&payload)
	if err != nil {
		http.Error(w, fmt.Sprintf("error parsing JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	switch payload.Type {
	case "PodResetRestarts":
		s.store.Dispatch(
			store.NewPodResetRestartsAction(payload.PodID, payload.ManifestName, payload.VisibleRestarts))
	default:
		http.Error(w, fmt.Sprintf("Unknown action type: %s", payload.Type), http.StatusBadRequest)
	}

}

func (s *HeadsUpServer) HandleTrigger(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "must be POST request", http.StatusBadRequest)
		return
	}

	var payload triggerPayload

	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&payload)
	if err != nil {
		http.Error(w, fmt.Sprintf("error parsing JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	if len(payload.ManifestNames) != 1 {
		http.Error(w, fmt.Sprintf("/api/trigger currently supports exactly one manifest name, got %d", len(payload.ManifestNames)), http.StatusBadRequest)
		return
	}

	err = MaybeSendToTriggerQueue(s.store, payload.ManifestNames[0])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func MaybeSendToTriggerQueue(st store.RStore, name string) error {
	mName := model.ManifestName(name)

	state := st.RLockState()
	m, ok := state.Manifest(mName)
	st.RUnlockState()

	if !ok {
		return fmt.Errorf("no manifest found with name '%s'", mName)
	}

	if m.TriggerMode != model.TriggerModeManual {
		return fmt.Errorf("can only trigger updates for manifests of TriggerModeManual")
	}

	st.Dispatch(AppendToTriggerQueueAction{Name: mName})
	return nil
}

/* -- SNAPSHOT: SENDING SNAPSHOT TO SERVER -- */
type snapshotURLJson struct {
	Url string `json:"url"`
}

// the default json decoding just blows up if a time.Time field is empty
// this uses the default behavior, except empty string -> time.Time{}
type timeAllowEmptyDecoder struct{}

func (codec timeAllowEmptyDecoder) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	s := iter.ReadString()
	var ret time.Time
	if s != "" {
		var err error
		ret, err = time.Parse(time.RFC3339, s)
		if err != nil {
			iter.ReportError("timeAllowEmptyDecoder", errors.Wrapf(err, "decoding '%s'", s).Error())
			return
		}
	}
	*((*time.Time)(ptr)) = ret
}

func (s *HeadsUpServer) HandleNewSnapshot(w http.ResponseWriter, req *http.Request) {
	st := s.store.RLockState()
	token := st.Token
	teamID := st.TeamName
	s.store.RUnlockState()

	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		msg := fmt.Sprintf("error reading body: %v", err)
		log.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	c := jsoniter.ConfigDefault
	de := jsoniter.DecoderExtension{reflect2.TypeOf(time.Time{}): timeAllowEmptyDecoder{}}
	c.RegisterExtension(de)
	var snapshot cloud.Snapshot
	decoder := c.NewDecoder(bytes.NewBuffer(b))

	// TODO(nick): Add more strict decoding once we have better safeguards for making
	// sure the Go and JS types are in-sync.
	// decoder.DisallowUnknownFields()

	err = decoder.Decode(&snapshot)
	if err != nil {
		msg := fmt.Sprintf("Error decoding snapshot: %v\n", err)
		log.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	id, err := s.uploader.Upload(token, teamID, snapshot)
	if err != nil {
		msg := fmt.Sprintf("Error creating snapshot: %v", err)
		log.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	responsePayload := snapshotURLJson{
		Url: s.uploader.IDToSnapshotURL(id),
	}

	//encode URL to JSON format
	urlJS, err := json.Marshal(responsePayload)
	if err != nil {
		msg := fmt.Sprintf("Error to marshal url JSON response %v", err)
		log.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	//write URL to header
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(urlJS)
	if err != nil {
		msg := fmt.Sprintf("Error writing URL response: %v", err)
		log.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

}

func (s *HeadsUpServer) GetView(ctx context.Context, req *proto_webview.GetViewRequest) (*proto_webview.View, error) {
	state := s.store.RLockState()
	view := webview.StateToProtoView(state)
	s.store.RUnlockState()

	return view, nil
}

func (s *HeadsUpServer) userStartedTiltCloudRegistration(w http.ResponseWriter, req *http.Request) {
	s.store.Dispatch(store.UserStartedTiltCloudRegistrationAction{})
}
