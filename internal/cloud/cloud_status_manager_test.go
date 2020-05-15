package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/httptest"
)

const testCloudAddress = "tiltcloud.example.com"

func TestLongPost(t *testing.T) {
	f := newCloudStatusManagerTestFixture(t)

	f.httpClient.SetResponse(`{"foo": "bar"}`)
	f.Run(func(state *store.EngineState) {
		state.CloudStatus.WaitingForStatusPostRegistration = true
	})

	f.waitForRequest(fmt.Sprintf("https://%s/api/whoami?wait_for_registration=true", testCloudAddress))
}

func TestWhoAmI(t *testing.T) {
	for _, tc := range []struct {
		name   string
		teamID string
	}{
		{"without team id", ""},
		{"with team id", "test team id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCloudStatusManagerTestFixture(t)

			resp := whoAmIResponse{
				Found:                true,
				Username:             "myusername",
				SuggestedTiltVersion: "10.0.0",
			}

			if tc.teamID != "" {
				resp.TeamName = "test team name"
			}

			respBytes, err := json.Marshal(resp)
			require.NoError(t, err)

			f.httpClient.SetResponse(string(respBytes))
			f.Run(func(state *store.EngineState) {
				state.TeamID = tc.teamID
				state.Token = "test token"
				state.TiltBuildInfo.Version = "test tilt version"
			})
			req := f.waitForRequest(fmt.Sprintf("https://%s/api/whoami", testCloudAddress))
			require.Equal(t, "test token", req.Header.Get(TiltTokenHeaderName))

			if tc.teamID == "" {
				_, ok := req.Header[http.CanonicalHeaderKey(TiltTeamIDNameHeaderName)]
				require.Falsef(t, ok, "request should not have header %s", TiltTeamIDNameHeaderName)
			} else {
				require.Equalf(t, "test team id", req.Header.Get(TiltTeamIDNameHeaderName), "header %s", TiltTeamIDNameHeaderName)
			}

			var j whoAmIRequest
			err = json.NewDecoder(req.Body).Decode(&j)
			require.NoError(t, err)
			require.Equal(t, "test tilt version", j.TiltVersion)

			expectedAction := store.TiltCloudStatusReceivedAction{
				Found:                    true,
				Username:                 "myusername",
				IsPostRegistrationLookup: false,
				SuggestedTiltVersion:     "10.0.0",
			}

			if tc.teamID != "" {
				expectedAction.TeamName = "test team name"
			}

			a := store.WaitForAction(t, reflect.TypeOf(store.TiltCloudStatusReceivedAction{}), f.st.Actions).(store.TiltCloudStatusReceivedAction)
			require.Equal(t, expectedAction, a)
		})
	}
}

func TestStatusRefresh(t *testing.T) {
	f := newCloudStatusManagerTestFixture(t)

	f.httpClient.SetResponse(`{"Username": "user1", "Found": true, "SuggestedTiltVersion": "10.0.0"}`)
	f.Run(func(state *store.EngineState) {
		state.Token = "test token"
		state.TiltBuildInfo.Version = "test tilt version"
	})

	req := f.waitForRequest(fmt.Sprintf("https://%s/api/whoami", testCloudAddress))
	require.Equal(t, "test token", req.Header.Get(TiltTokenHeaderName))

	expected := store.TiltCloudStatusReceivedAction{Username: "user1", Found: true, IsPostRegistrationLookup: false, SuggestedTiltVersion: "10.0.0"}
	a := store.WaitForAction(t, reflect.TypeOf(store.TiltCloudStatusReceivedAction{}), f.st.Actions)
	require.Equal(t, expected, a)

	f.st.ClearActions()

	// check that setting a team id triggers a refresh
	f.httpClient.SetResponse(`{"Username": "user2", "Found": true}`)
	f.Run(func(state *store.EngineState) {
		state.TeamID = "test team id"
		state.Token = "test token"
		state.TiltBuildInfo.Version = "test tilt version"
	})

	req = f.waitForRequest(fmt.Sprintf("https://%s/api/whoami", testCloudAddress))
	require.Equal(t, "test token", req.Header.Get(TiltTokenHeaderName))

	expected = store.TiltCloudStatusReceivedAction{Username: "user2", Found: true, IsPostRegistrationLookup: false}
	a = store.WaitForAction(t, reflect.TypeOf(store.TiltCloudStatusReceivedAction{}), f.st.Actions)
	require.Equal(t, expected, a)

	f.st.ClearActions()

	f.um.OnChange(f.ctx, f.st)
	store.AssertNoActionOfType(t, reflect.TypeOf(store.TiltCloudStatusReceivedAction{}), f.st.Actions)

	// check that we periodically refresh
	f.clock.Advance(24 * time.Hour)

	f.um.OnChange(f.ctx, f.st)
	a = store.WaitForAction(t, reflect.TypeOf(store.TiltCloudStatusReceivedAction{}), f.st.Actions)
	require.Equal(t, expected, a)
}

type cloudStatusManagerTestFixture struct {
	um         *CloudStatusManager
	httpClient *httptest.FakeClient
	st         *store.TestingStore
	ctx        context.Context
	t          *testing.T
	clock      clockwork.FakeClock
}

func newCloudStatusManagerTestFixture(t *testing.T) *cloudStatusManagerTestFixture {
	st := store.NewTestingStore()

	httpClient := httptest.NewFakeClient()

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()

	clock := clockwork.NewFakeClock()

	return &cloudStatusManagerTestFixture{
		st:         st,
		httpClient: httpClient,
		clock:      clock,
		um:         NewStatusManager(httpClient, clock),
		ctx:        ctx,
		t:          t,
	}
}

func (f *cloudStatusManagerTestFixture) Run(mutateState func(state *store.EngineState)) {
	state := store.EngineState{
		CloudAddress: testCloudAddress,
	}
	mutateState(&state)
	f.st.SetState(state)
	f.um.OnChange(f.ctx, f.st)
}

func (f *cloudStatusManagerTestFixture) waitForRequest(expectedURL string) http.Request {
	timeout := time.After(time.Second)
	for {
		reqs := f.httpClient.Requests()
		if len(reqs) > 1 {
			var urls []string
			for _, req := range reqs {
				urls = append(urls, req.URL.String())
			}
			f.t.Fatalf("%T made more than one http request! requests: %v", f.um, urls)
		} else if len(reqs) == 1 {
			ret := reqs[0]
			require.Equal(f.t, expectedURL, ret.URL.String())
			require.Equal(f.t, "POST", ret.Method)
			f.httpClient.ClearRequests()
			return ret
		} else {
			select {
			case <-timeout:
				f.t.Fatalf("timed out waiting for %T to make http request", f.um)
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
}
