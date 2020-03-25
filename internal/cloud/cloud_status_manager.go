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

	"github.com/windmilleng/tilt/internal/cloud/cloudurl"
	"github.com/windmilleng/tilt/internal/feature"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
)

// to avoid infinitely resubmitting requests on error
const timeoutAfterError = 5 * time.Minute

const TiltTokenHeaderName = "X-Tilt-Token"
const TiltTeamIDNameHeaderName = "X-Tilt-TeamID"

func NewStatusManager(client HttpClient) *CloudStatusManager {
	return &CloudStatusManager{client: client}
}

type CloudStatusManager struct {
	client HttpClient

	sleepingAfterErrorUntil time.Time
	currentlyMakingRequest  bool
	mu                      sync.Mutex
}

func ProvideHttpClient() HttpClient {
	return http.DefaultClient
}

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type whoAmIResponse struct {
	Found    bool
	Username string
}

func (c *CloudStatusManager) error() {
	c.mu.Lock()
	c.sleepingAfterErrorUntil = time.Now().Add(timeoutAfterError)
	c.mu.Unlock()
}

type whoAmIRequest struct {
	TiltVersion string `json:"tilt_version"`
}

func (c *CloudStatusManager) CheckStatus(ctx context.Context, st store.RStore, blocking bool) {
	state := st.RLockState()
	tok := state.Token
	teamID := state.TeamID
	tiltVersion := state.TiltBuildInfo.Version
	st.RUnlockState()

	c.mu.Lock()
	c.currentlyMakingRequest = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.currentlyMakingRequest = false
		c.mu.Unlock()
	}()

	u := cloudurl.URL(state.CloudAddress)
	u.Path = "/api/whoami"

	if blocking {
		q := url.Values{}
		q.Set("wait_for_registration", "true")
		u.RawQuery = q.Encode()
	}

	body := &bytes.Buffer{}
	err := json.NewEncoder(body).Encode(whoAmIRequest{TiltVersion: tiltVersion})
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
	req.Header.Set(TiltTokenHeaderName, string(tok))
	if teamID != "" {
		req.Header.Set(TiltTeamIDNameHeaderName, teamID)
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

	st.Dispatch(store.TiltCloudStatusReceivedAction{
		Found:                    r.Found,
		Username:                 r.Username,
		IsPostRegistrationLookup: blocking,
	})
}

func (c *CloudStatusManager) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	defer st.RUnlockState()

	if !state.Features[feature.Snapshots] {
		return
	}

	c.mu.Lock()
	sleepingAfterErrorUntil := c.sleepingAfterErrorUntil
	currentlyMakingRequest := c.currentlyMakingRequest
	c.mu.Unlock()

	if state.CloudStatus.WaitingForStatusPostRegistration && !currentlyMakingRequest {
		go c.CheckStatus(ctx, st, true)
		return
	}

	// c.currentlyMakingRequest is a bit of a race condition here:
	// 1. start making request that's going to return TokenKnownUnregistered = true
	// 2. before request finishes, web ui triggers refresh, setting TokenKnownUnregistered = false
	// 3. request started in (1) finishes, sets TokenKnownUnregistered = true
	// we never make a request post-(2), where the token was registered
	// This is mitigated by - a) the window between (1) and (3) is small, and b) the user can just click refresh again
	if time.Now().Before(sleepingAfterErrorUntil) ||
		state.CloudStatus.Username != "" ||
		state.CloudStatus.TokenKnownUnregistered ||
		currentlyMakingRequest {
		return
	}

	go c.CheckStatus(ctx, st, false)
}
