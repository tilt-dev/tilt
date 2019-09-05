package cloud

import (
	"context"
	"fmt"
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

func (f *cloudUsernameManagerTestFixture) waitForRequest(expectedURL string) {
	timeout := time.After(time.Second)
	for {
		urls := f.httpClient.RequestURLs()
		if len(urls) > 1 {
			f.t.Fatalf("%T made more than one http request! requests: %v", f.um, urls)
		} else if len(urls) == 1 {
			require.Equal(f.t, expectedURL, urls[0])
			return
		} else {
			select {
			case <-timeout:
				f.t.Fatalf("timed out waiting for %T to make http request", f.um)
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
}
