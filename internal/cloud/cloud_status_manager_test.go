package cloud

import (
	"context"
	"encoding/json"
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

func TestWhoAmI(t *testing.T) {
	f := newCloudStatusManagerTestFixture(t)

	resp := whoAmIResponse{
		SuggestedTiltVersion: "10.0.0",
	}

	respBytes, err := json.Marshal(resp)
	require.NoError(t, err)

	f.httpClient.SetResponse(string(respBytes))
	f.Run(func(state *store.EngineState) {
		state.TiltBuildInfo.Version = "test tilt version"
	})

	expectedAction := store.TiltCloudStatusReceivedAction{
		SuggestedTiltVersion: "10.0.0",
	}

	a := store.WaitForAction(t, reflect.TypeOf(store.TiltCloudStatusReceivedAction{}), f.st.Actions).(store.TiltCloudStatusReceivedAction)
	require.Equal(t, expectedAction, a)
}

func TestStatusRefresh(t *testing.T) {
	f := newCloudStatusManagerTestFixture(t)

	f.httpClient.SetResponse(`{"Username": "user1", "Found": true, "SuggestedTiltVersion": "10.0.0"}`)
	f.Run(func(state *store.EngineState) {
		state.TiltBuildInfo.Version = "test tilt version"
	})

	expected := store.TiltCloudStatusReceivedAction{SuggestedTiltVersion: "10.0.0"}
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

	expected = store.TiltCloudStatusReceivedAction{}
	a = store.WaitForAction(t, reflect.TypeOf(store.TiltCloudStatusReceivedAction{}), f.st.Actions)
	require.Equal(t, expected, a)

	f.st.ClearActions()

	_ = f.um.OnChange(f.ctx, f.st, store.LegacyChangeSummary())
	store.AssertNoActionOfType(t, reflect.TypeOf(store.TiltCloudStatusReceivedAction{}), f.st.Actions)

	// check that we periodically refresh
	f.clock.Advance(24 * time.Hour)

	_ = f.um.OnChange(f.ctx, f.st, store.LegacyChangeSummary())
	a = store.WaitForAction(t, reflect.TypeOf(store.TiltCloudStatusReceivedAction{}), f.st.Actions)
	require.Equal(t, expected, a)
}

type cloudStatusManagerTestFixture struct {
	um         *CloudStatusManager
	httpClient *httptest.FakeClient
	st         *store.TestingStore
	ctx        context.Context
	t          *testing.T
	clock      *clockwork.FakeClock
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
	_ = f.um.OnChange(f.ctx, f.st, store.LegacyChangeSummary())
}
