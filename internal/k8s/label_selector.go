package k8s

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
)

func setEqual(a, b sets.String) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		if b[k] != v {
			return false
		}
	}

	return true
}

func requirementEqual(a, b labels.Requirement) bool {
	return a.Key() == b.Key() &&
		a.Operator() == b.Operator() &&
		setEqual(a.Values(), b.Values())
}

func requirementsEqual(a, b labels.Requirements) bool {
	if len(a) != len(b) {
		return false
	}

	for i, e1 := range a {
		if !requirementEqual(e1, b[i]) {
			return false
		}
	}

	return true
}

func SelectorEqual(a, b labels.Selector) bool {
	reqA, selA := a.Requirements()
	reqB, selB := b.Requirements()
	return selA == selB && requirementsEqual(reqA, reqB)
}
