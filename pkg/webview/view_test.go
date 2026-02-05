package webview

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/timecmp"
	v1alpha1 "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestEncodeTimeJSON(t *testing.T) {
	now := time.Unix(1620234922, 2000000)
	time := metav1.NewTime(now)
	assert.Equal(t, `"2021-05-05T17:15:22Z"`, encode(t, time))
	assert.Equal(t, `"2021-05-05T17:15:22Z"`, encode(t, &time))
}

func TestDecodeUIBuildRunning(t *testing.T) {
	build := v1alpha1.UIBuildRunning{}
	decode(t, `{
  "startTime": "2021-05-05T17:15:22.002000Z"
}`, &build)

	timecmp.AssertTimeEqual(t, build.StartTime, time.Unix(1620234922, 2000000))
}

func TestEncodeUIBuildRunning(t *testing.T) {
	now := time.Unix(1620234922, 2000000)
	build := v1alpha1.UIBuildRunning{StartTime: metav1.NewMicroTime(now)}
	assert.Equal(t, `{"startTime":"2021-05-05T17:15:22.002000Z"}`, encode(t, build))
	assert.Equal(t, `{"startTime":"2021-05-05T17:15:22.002000Z"}`, encode(t, &build))
}

func TestEncodeMicroTimeJSON(t *testing.T) {
	now := time.Unix(1620234922, 2000000)
	time := metav1.NewMicroTime(now)
	assert.Equal(t, `"2021-05-05T17:15:22.002000Z"`, encode(t, time))
	assert.Equal(t, `"2021-05-05T17:15:22.002000Z"`, encode(t, &time))
}

func encode(t *testing.T, obj interface{}) string {
	buf := bytes.NewBuffer(nil)
	encoder := json.NewEncoder(buf)
	require.NoError(t, encoder.Encode(obj))
	return strings.TrimSpace(buf.String())
}

func decode(t *testing.T, content string, v interface{}) {
	require.NoError(t, json.NewDecoder(strings.NewReader(content)).Decode(v))
}
