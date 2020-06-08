package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/tilt-dev/tilt/internal/cloud/cloudurl"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/token"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// to avoid infinitely resubmitting requests on error
const timeoutAfterError = 5 * time.Minute

// how frequently we'll refresh cloud status, even if nothing changes
const refreshPeriod = time.Hour

const TiltTokenHeaderName = "X-Tilt-Token"
const TiltTeamIDNameHeaderName = "X-Tilt-TeamID"

func NewStatusManager(client HttpClient, clock clockwork.Clock) *CloudStatusManager {
	return &CloudStatusManager{client: client, clock: clock}
}

// if any of these fields change, we know we need to do a fresh lookup
type statusRequestKey struct {
	tiltToken token.Token
	teamID    string
	version   model.TiltBuild
}

type CloudStatusManager struct {
	client HttpClient
	clock  clockwork.Clock

	mu sync.Mutex

	lastErrorTime          time.Time
	currentlyMakingRequest bool

	lastRequestKey       statusRequestKey
	lastSuccessfulLookup time.Time
}

func ProvideHttpClient() HttpClient {
	return http.DefaultClient
}

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type whoAmIResponse struct {
	Found                bool
	Username             string
	TeamName             string
	SuggestedTiltVersion string
}

func (c *CloudStatusManager) error() {
	c.mu.Lock()
	c.lastErrorTime = c.clock.Now()
	c.mu.Unlock()
}

type whoAmIRequest struct {
	TiltVersion string `json:"tilt_version"`
}

func (c *CloudStatusManager) CheckStatus(ctx context.Context, st store.RStore, cloudAddress string, requestKey statusRequestKey, blocking bool) {
	c.mu.Lock()
	c.currentlyMakingRequest = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.currentlyMakingRequest = false
		c.mu.Unlock()
	}()

	u := cloudurl.URL(cloudAddress)
	u.Path = "/api/whoami"

	if blocking {
		q := url.Values{}
		q.Set("wait_for_registration", "true")
		u.RawQuery = q.Encode()
	}

	body := &bytes.Buffer{}
	err := json.NewEncoder(body).Encode(whoAmIRequest{TiltVersion: requestKey.version.Version})
	if err != nil {
		logger.Get(ctx).Debugf("error serializing whoami request: %v\n", err)
		c.error()
		return
	}

	req, err := http.NewRequest("POST", u.String(), body)
	if err != nil {
		logger.Get(ctx).Debugf("error making whoami request: %v", err)
		c.error()
		return
	}
	req.Header.Set(TiltTokenHeaderName, string(requestKey.tiltToken))
	if requestKey.teamID != "" {
		req.Header.Set(TiltTeamIDNameHeaderName, requestKey.teamID)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client.Do(req)
	if err != nil {
		logger.Get(ctx).Debugf("error checking tilt cloud status: %v", err)
		c.error()
		return
	}

	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logger.Get(ctx).Debugf("tilt cloud status request failed with status %d. error reading response body: %v", resp.StatusCode, err)
			c.error()
			return
		}
		logger.Get(ctx).Debugf("error checking tilt cloud status: code: %d, message: %s", resp.StatusCode, string(body))
		c.error()
		return
	}

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Get(ctx).Debugf("error reading response body: %v", err)
		c.error()
		return
	}
	r := whoAmIResponse{}
	err = json.NewDecoder(bytes.NewReader(responseBody)).Decode(&r)
	if err != nil {
		logger.Get(ctx).Debugf("error decoding tilt whoami response '%s': %v", string(responseBody), err)
		c.error()
		return
	}

	c.mu.Lock()
	c.lastRequestKey = requestKey
	c.lastSuccessfulLookup = c.clock.Now()
	c.lastErrorTime = time.Time{}
	c.mu.Unlock()

	st.Dispatch(store.TiltCloudStatusReceivedAction{
		Found:                    r.Found,
		Username:                 r.Username,
		TeamName:                 r.TeamName,
		IsPostRegistrationLookup: blocking,
		SuggestedTiltVersion:     r.SuggestedTiltVersion,
	})
}

func (c *CloudStatusManager) needsLookup(requestKey statusRequestKey) bool {
	return c.lastSuccessfulLookup.IsZero() ||
		c.lastSuccessfulLookup.Add(refreshPeriod).Before(c.clock.Now()) ||
		requestKey != c.lastRequestKey
}

func (c *CloudStatusManager) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	defer st.RUnlockState()

	c.mu.Lock()
	lastErrorTime := c.lastErrorTime
	currentlyMakingRequest := c.currentlyMakingRequest
	requestKey := statusRequestKey{teamID: state.TeamID, tiltToken: state.Token, version: state.TiltBuildInfo}
	needsLookup := c.needsLookup(requestKey)
	c.mu.Unlock()

	if state.CloudStatus.WaitingForStatusPostRegistration && !currentlyMakingRequest {
		go c.CheckStatus(ctx, st, state.CloudAddress, requestKey, true)
		return
	}

	// c.currentlyMakingRequest is a bit of a race condition here:
	// 1. start making request that's going to return TokenKnownUnregistered = true
	// 2. before request finishes, web ui triggers refresh, setting TokenKnownUnregistered = false
	// 3. request started in (1) finishes, sets TokenKnownUnregistered = true
	// we never make a request post-(2), where the token was registered
	// This is mitigated by - a) the window between (1) and (3) is small, and b) the user can just click refresh again
	allowedToPerformLookup := !time.Now().Before(lastErrorTime.Add(timeoutAfterError)) && !currentlyMakingRequest

	if needsLookup && allowedToPerformLookup {
		go c.CheckStatus(ctx, st, state.CloudAddress, requestKey, false)
		return
	}
}
