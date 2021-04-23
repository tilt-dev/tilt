package k8s

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
)

func TestUIDSet(t *testing.T) {
	uids := NewUIDSet()
	require.NotNil(t, uids)
	require.Equal(t, 0, len(uids))

	values := []types.UID{types.UID("uid-0"), types.UID("uid-1"), types.UID("uid-2"), types.UID("uid-3")}
	for _, v := range values {
		require.False(t, uids.Contains(v))
	}
	uids.Add(values[0])
	require.True(t, uids.Contains("uid-0"))
	uids.Add(values[1], values[2])
	for _, v := range values[:3] {
		require.True(t, uids.Contains(v))
	}
	require.False(t, uids.Contains(values[3]))
}
