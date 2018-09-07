package ignore

type CompositeIgnoreTester struct {
	Testers []Tester
}

func (c CompositeIgnoreTester) IsIgnored(f string, isDir bool) (bool, error) {
	for _, t := range c.Testers {
		ret, err := t.IsIgnored(f, isDir)
		if err != nil {
			return false, err
		}
		if ret {
			return true, nil
		}
	}
	return false, nil
}

var _ Tester = CompositeIgnoreTester{}
