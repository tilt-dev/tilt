package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/feature"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/httptest"
)

const testCloudAddress = "tiltcloud.example.com"

func TestLongGet(t *testing.T) {
	f := newCloudUsernameManagerTestFixture(t)

	f.httpClient.SetResponse(`{"foo": "bar"}`)
	f.Run(func(state *store.EngineState) {
		state.WaitingForTiltCloudUsernamePostRegistration = true
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
			f := newCloudUsernameManagerTestFixture(t)

			f.httpClient.SetResponse(`{"foo": "bar"}`)
			f.Run(func(state *store.EngineState) {
				state.TeamName = tc.teamID
				state.Token = "test token"
				state.TiltBuildInfo.Version = "test tilt version"
			})
			req := f.waitForRequest(fmt.Sprintf("https://%s/api/whoami", testCloudAddress))
			require.Equal(t, "test token", req.Header.Get(TiltTokenHeaderName))

			if tc.teamID == "" {
				_, ok := req.Header[TiltTeamIDNameHeaderName]
				require.Falsef(t, ok, "request should not have header %s", TiltTeamIDNameHeaderName)
			} else {
				require.Equalf(t, "test team id", req.Header.Get(TiltTeamIDNameHeaderName), "header %s", TiltTeamIDNameHeaderName)
			}

			var j whoAmIRequest
			err := json.NewDecoder(req.Body).Decode(&j)
			require.NoError(t, err)
			require.Equal(t, "test tilt version", j.TiltVersion)
		})
	}
}

type cloudUsernameManagerTestFixture struct {
	um         *CloudUsernameManager
	httpClient *httptest.FakeClient
	st         *store.Store
	ctx        context.Context
	t          *testing.T
}

func newCloudUsernameManagerTestFixture(t *testing.T) *cloudUsernameManagerTestFixture {
	st, _ := store.NewStoreForTesting()

	httpClient := httptest.NewFakeClient()

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()

	return &cloudUsernameManagerTestFixture{
		st:         st,
		httpClient: httpClient,
		um:         NewUsernameManager(httpClient),
		ctx:        ctx,
		t:          t,
	}
}

func (f *cloudUsernameManagerTestFixture) Run(mutateState func(state *store.EngineState)) {
	state := f.st.LockMutableStateForTesting()
	state.Features = make(map[string]bool)
	state.Features[feature.Snapshots] = true
	state.CloudAddress = testCloudAddress
	mutateState(state)
	f.st.UnlockMutableState()
	f.um.OnChange(f.ctx, f.st)
}

func (f *cloudUsernameManagerTestFixture) waitForRequest(expectedURL string) http.Request {
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
			require.Equal(f.t, expectedURL, reqs[0].URL.String())
			return reqs[0]
		} else {
			select {
			case <-timeout:
				f.t.Fatalf("timed out waiting for %T to make http request", f.um)
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
}
