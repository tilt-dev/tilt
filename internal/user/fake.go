package user

func NewFakePrefs() *FakePrefs {
	return &FakePrefs{}
}

type FakePrefs struct {
	Prefs Prefs
}

func (i *FakePrefs) Get() (Prefs, error) {
	return i.Prefs, nil
}

func (i *FakePrefs) Update(newPrefs Prefs) error {
	i.Prefs = newPrefs
	return nil
}
