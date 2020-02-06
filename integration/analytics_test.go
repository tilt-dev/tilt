//+build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/wmclient/pkg/analytics"

	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

const WindmillDirEnvVarName = "WINDMILL_DIR"
const AnalyticsUrlEnvVarName = "TILT_ANALYTICS_URL"

type analyticsFixture struct {
	*k8sFixture
	tempDir *tempdir.TempDirFixture
	mss     *MemoryStatsServer
}

func newAnalyticsFixture(t *testing.T) *analyticsFixture {
	td := tempdir.NewTempDirFixture(t)
	af := &analyticsFixture{
		k8sFixture: newK8sFixture(t, "analytics"),
		tempDir:    td,
	}
	af.tilt.Environ[WindmillDirEnvVarName] = td.Path()

	af.SetupAnalyticsServer()

	return af
}

func (af *analyticsFixture) SetupAnalyticsServer() {
	mss, port, err := StartMemoryStatsServer()
	if !assert.NoError(af.t, err) {
		af.t.FailNow()
	}
	af.mss = mss
	af.tilt.Environ["TILT_DISABLE_ANALYTICS"] = ""
	af.tilt.Environ["CI"] = ""
	af.tilt.Environ[AnalyticsUrlEnvVarName] = fmt.Sprintf("http://localhost:%d/report", port)
}

func (af *analyticsFixture) TearDown() {
	err := af.mss.TearDown()
	if err != nil {
		af.t.Fatal(err)
	}
	af.tempDir.TearDown()
	af.k8sFixture.TearDown()
}

type envVarValue struct {
	name  string
	isSet bool
	val   string
}

func saveEnvVar(name string) envVarValue {
	val, isSet := os.LookupEnv(name)
	return envVarValue{
		name:  name,
		isSet: isSet,
		val:   val,
	}
}

func restoreEnvVar(v envVarValue) error {
	if !v.isSet {
		return os.Unsetenv(v.name)
	} else {
		return os.Setenv(v.name, v.val)
	}
}

func (af *analyticsFixture) SetOpt(opt analytics.Opt) {
	oldVal := saveEnvVar(WindmillDirEnvVarName)
	err := os.Setenv(WindmillDirEnvVarName, af.tempDir.Path())
	if err != nil {
		af.t.Fatal(err)
	}
	err = analytics.SetOpt(opt)
	if err != nil {
		af.t.Fatal(err)
	}
	err = restoreEnvVar(oldVal)
	if err != nil {
		af.t.Fatal(err)
	}
}

func TestOptedIn(t *testing.T) {
	f := newAnalyticsFixture(t)
	defer f.TearDown()

	f.SetOpt(analytics.OptIn)

	f.TiltUp("analytics")

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.WaitForAllPodsReady(ctx, "app=analytics")

	var observedEventNames []string
	for _, c := range f.mss.ma.Counts {
		observedEventNames = append(observedEventNames, c.Name)
	}

	var observedTimerNames []string
	for _, c := range f.mss.ma.Timers {
		observedTimerNames = append(observedTimerNames, c.Name)
	}

	// just check that a couple metrics were successfully reported rather than asserting an exhaustive list
	// the goal is to ensure that analytics is working in general, not to test which specific metrics are reported
	// and we don't want to have to update this every time we change which metrics we report
	assert.Contains(t, observedEventNames, "tilt.cmd.up")
	assert.Contains(t, observedEventNames, "tilt.tiltfile.loaded")
	assert.Contains(t, observedTimerNames, "tilt.tiltfile.load")
}

func TestOptedOut(t *testing.T) {
	f := newAnalyticsFixture(t)
	defer f.TearDown()

	f.SetOpt(analytics.OptOut)

	f.TiltUp("analytics")

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.WaitForAllPodsReady(ctx, "app=analytics")

	assert.Equal(t, 0, len(f.mss.ma.Counts))
}

func TestOptDefault(t *testing.T) {
	f := newAnalyticsFixture(t)
	defer f.TearDown()

	f.SetOpt(analytics.OptDefault)

	f.TiltUp("analytics")

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.WaitForAllPodsReady(ctx, "app=analytics")

	var observedEventNames []string
	for _, c := range f.mss.ma.Counts {
		observedEventNames = append(observedEventNames, c.Name)
	}

	var observedTimerNames []string
	for _, c := range f.mss.ma.Timers {
		observedTimerNames = append(observedTimerNames, c.Name)
	}

	// just check that a couple metrics were successfully reported rather than asserting an exhaustive list
	// the goal is to ensure that analytics is working in general, not to test which specific metrics are reported
	// and we don't want to have to update this every time we change which metrics we report
	assert.Contains(t, observedEventNames, "tilt.cmd.up")
	assert.Contains(t, observedEventNames, "tilt.tiltfile.loaded")
	assert.Contains(t, observedTimerNames, "tilt.tiltfile.load")
}
