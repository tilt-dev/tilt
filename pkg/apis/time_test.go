package apis_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis"
)

func TestMicroTime(t *testing.T) {
	now := apis.NowMicro()
	require.Equal(t, 0, now.Nanosecond()%int(time.Microsecond))
	d, err := json.Marshal(now)
	require.NoError(t, err)
	var out metav1.MicroTime
	require.NoError(t, json.Unmarshal(d, &out))
	// N.B. this is an intentional use of == operator against a time value
	// 	because in this case we really want to know that the struct is identical
	require.True(t, now == out,
		"Time did not round-trip\noriginal: %s\nafter: %s",
		now.String(), out.String())
}

func TestTime(t *testing.T) {
	now := apis.Now()
	require.Equal(t, 0, now.Nanosecond())
	d, err := json.Marshal(now)
	require.NoError(t, err)
	var out metav1.Time
	require.NoError(t, json.Unmarshal(d, &out))
	// N.B. this is an intentional use of == operator against a time value
	// 	because in this case we really want to know that the struct is identical
	require.True(t, now == out,
		"Time did not round-trip\noriginal: %s\nafter: %s",
		now.String(), out.String())
}
