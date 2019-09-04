package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/gorilla/websocket"
	"github.com/windmilleng/wmclient/pkg/analytics"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/cloud"
	"github.com/windmilleng/tilt/internal/hud/webview"
	"github.com/windmilleng/tilt/internal/sail/client"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/assets"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
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

type HeadsUpServer struct {
	store             *store.Store
	router            *mux.Router
	a                 *tiltanalytics.TiltAnalytics
	sailCli           client.SailClient
	numWebsocketConns int32
	httpCli           httpClient
	cloudAddress      string
}

func ProvideHeadsUpServer(store *store.Store, assetServer assets.Server, analytics *tiltanalytics.TiltAnalytics, sailCli client.SailClient, httpClient httpClient, cloudAddress cloud.Address) *HeadsUpServer {
	r := mux.NewRouter().UseEncodedPath()
	s := &HeadsUpServer{
		store:        store,
		router:       r,
		a:            analytics,
		sailCli:      sailCli,
		httpCli:      httpClient,
		cloudAddress: string(cloudAddress),
	}

	r.HandleFunc("/api/view", s.ViewJSON)
	r.HandleFunc("/api/analytics", s.HandleAnalytics)
	r.HandleFunc("/api/analytics_opt", s.HandleAnalyticsOpt)
	r.HandleFunc("/api/sail", s.HandleSail)
	r.HandleFunc("/api/trigger", s.HandleTrigger)
	r.HandleFunc("/api/snapshot/new", s.HandleNewSnapshot).Methods("POST")
	// this endpoint is only used for testing snapshots in development
	r.HandleFunc("/api/snapshot/{snapshot_id}", s.SnapshotJSON)
	r.HandleFunc("/ws/view", s.ViewWebsocket)
	r.HandleFunc("/api/user_started_tilt_cloud_registration", s.userStartedTiltCloudRegistration)

	r.PathPrefix("/").Handler(s.cookieWrapper(assetServer))

	return s
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
	view := webview.StateToWebView(state)
	s.store.RUnlockState()

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(view)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error rendering view payload: %v", err), http.StatusInternalServerError)
	}
}

type snapshot struct {
	View webview.View
}

func (s *HeadsUpServer) SnapshotJSON(w http.ResponseWriter, req *http.Request) {
	state := s.store.RLockState()
	view := webview.StateToWebView(state)
	s.store.RUnlockState()

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(snapshot{
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

func (s *HeadsUpServer) HandleSail(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "must be POST request", http.StatusBadRequest)
		return
	}

	// Request context doesn't have logger, just slap one on for now.
	l := logger.NewFuncLogger(false, logger.DebugLvl, func(level logger.Level, b []byte) error {
		s.store.Dispatch(store.NewGlobalLogEvent(b))
		return nil
	})

	err := s.sailCli.Connect(logger.WithLogger(req.Context(), l), s.store)
	if err != nil {
		log.Printf("sailClient.NewRoom: %v", err)
		http.Error(w, fmt.Sprintf("error creating new Sail room: %v", err), http.StatusInternalServerError)
		return
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
type SnapshotID string

type snapshotIDResponse struct {
	ID string
}

func (s *HeadsUpServer) templateSnapshotURL(id SnapshotID) string {
	u := cloud.URL(s.cloudAddress)
	u.Path = fmt.Sprintf("snapshot/%s", id)
	return u.String()
}

func (s *HeadsUpServer) newSnapshotURL() string {
	u := cloud.URL(s.cloudAddress)
	u.Path = "/api/snapshot/new"
	return u.String()
}

func (s *HeadsUpServer) HandleNewSnapshot(w http.ResponseWriter, req *http.Request) {
	st := s.store.RLockState()
	token := st.Token
	s.store.RUnlockState()

	request, err := http.NewRequest(http.MethodPost, s.newSnapshotURL(), req.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error making request: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	request.Header.Set(cloud.TiltTokenHeaderName, token.String())
	response, err := s.httpCli.Do(request)
	if err != nil {
		log.Printf("Error creating snapshot: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(err.Error()))
		if err != nil {
			log.Printf("Error writing error to response: %v\n", err)
		}
		return
	}

	responseWithID, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Printf("Error reading response when creating snapshot: %v\n", err)
		log.Printf("Error reading responseWithID: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	//unpack response with snapshot ID
	var resp snapshotIDResponse
	err = json.Unmarshal(responseWithID, &resp)
	if err != nil || resp.ID == "" {
		log.Printf("Error unpacking snapshot response JSON: %v\nJSON: %s\n", err, responseWithID)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	//create URL with snapshot ID
	var ID SnapshotID
	ID = SnapshotID(resp.ID)
	responsePayload := snapshotURLJson{
		Url: s.templateSnapshotURL(ID),
	}

	//encode URL to JSON format
	urlJS, err := json.Marshal(responsePayload)
	if err != nil {
		log.Printf("Error to marshal url JSON response %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	//write URL to header
	w.WriteHeader(response.StatusCode)
	_, err = w.Write(urlJS)
	if err != nil {
		log.Printf("Error writing URL response: %v\n", err)
		return
	}

}

func (s *HeadsUpServer) userStartedTiltCloudRegistration(w http.ResponseWriter, req *http.Request) {
	s.store.Dispatch(store.UserStartedTiltCloudRegistrationAction{})
}

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

func ProvideHttpClient() httpClient {
	return http.DefaultClient
}
