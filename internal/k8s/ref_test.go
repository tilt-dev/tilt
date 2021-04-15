package k8s

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func TestObjRefList(t *testing.T) {
	var orl ObjRefList
	require.False(t, orl.ContainsUID("abc123"))
	require.Empty(t, orl.UIDSet())
	ref, ok := orl.GetByUID("abc123")
	require.False(t, ok)
	require.Empty(t, ref)

	ref = v1.ObjectReference{UID: "abc123", Namespace: "namespace", Name: "name"}
	orl = append(orl, ref)
	require.True(t, orl.ContainsUID("abc123"))
	require.False(t, orl.ContainsUID("def456"))
	expectedUIDs := NewUIDSet()
	expectedUIDs.Add("abc123")
	require.Equal(t, expectedUIDs, orl.UIDSet())

	r, ok := orl.GetByUID("abc123")
	require.True(t, ok)
	require.Equal(t, ref, r)
}
