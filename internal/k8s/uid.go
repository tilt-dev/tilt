package k8s

import "k8s.io/apimachinery/pkg/types"

type UIDSet map[types.UID]bool

func NewUIDSet(uids ...types.UID) UIDSet {
	ret := make(map[types.UID]bool)
	for _, uid := range uids {
		ret[uid] = true
	}
	return ret
}

func (s UIDSet) Add(uids ...types.UID) {
	for _, uid := range uids {
		s[uid] = true
	}
}

func (s UIDSet) Contains(uid types.UID) bool {
	return s[uid]
}
