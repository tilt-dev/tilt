package webview

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/timecmp"
	v1alpha1 "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Time serialization is a mess.
//
// There are basically two competing time serialization standards
// that apimachinery uses.
//
// RFC 3339 (https://tools.ietf.org/html/rfc3339)
// apimachinery uses this to serialize time in JSON
// https://github.com/kubernetes/apimachinery/blob/v0.21.0/pkg/apis/meta/v1/time.go#L141
//
// protoreflect (the new Go V2 version of protobufs), which
// insists on creating structs and using reflection.
// apimachinery uses this to serialize time as (second, nanosecond) pairs
// https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Timestamp
//
// These blow up jsonpb, which sometimes goes down the protoreflect
// codepath and sometimes goes down the RFC3339 codepath.
// Instead, we hack everything to serialize as RFC3339.

func TestEncodeTimeJSONPB(t *testing.T) {
	now := time.Unix(1620234922, 2000000)
	time := metav1.NewTime(now)
	assert.Equal(t, `"2021-05-05T17:15:22Z"
`, encode(t, time))
	assert.Equal(t, `"2021-05-05T17:15:22Z"
`, encode(t, &time))
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
	assert.Equal(t, `{"startTime":"2021-05-05T17:15:22.002000Z"}
`, encode(t, build))
	assert.Equal(t, `{
  "startTime": "2021-05-05T17:15:22.002000Z"
}
`, encode(t, &build))
}

func TestEncodeMicroTimeJSONPB(t *testing.T) {
	now := time.Unix(1620234922, 2000000)
	time := metav1.NewMicroTime(now)
	assert.Equal(t, `"2021-05-05T17:15:22.002000Z"
`, encode(t, time))
	assert.Equal(t, `"2021-05-05T17:15:22.002000Z"
`, encode(t, &time))
}

func encode(t *testing.T, obj interface{}) string {
	buf := bytes.NewBuffer(nil)
	jsEncoder := &runtime.JSONPb{
		OrigName: false,
		Indent:   "  ",
	}
	require.NoError(t, jsEncoder.NewEncoder(buf).Encode(obj))
	return buf.String()
}

func decode(t *testing.T, content string, v interface{}) {
	jsEncoder := &runtime.JSONPb{
		OrigName: false,
		Indent:   "  ",
	}
	require.NoError(t, jsEncoder.NewDecoder(strings.NewReader(content)).Decode(v))
}
