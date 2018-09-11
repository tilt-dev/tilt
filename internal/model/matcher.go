package model

type PathMatcher interface {
	Matches(f string, isDir bool) (bool, error)
}

type CompositePathMatcher struct {
	Matchers []PathMatcher
}

func (c CompositePathMatcher) Matches(f string, isDir bool) (bool, error) {
	for _, t := range c.Matchers {
		ret, err := t.Matches(f, isDir)
		if err != nil {
			return false, err
		}
		if ret {
			return true, nil
		}
	}
	return false, nil
}

var _ PathMatcher = CompositePathMatcher{}
