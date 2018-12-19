package model

type resourceInfo interface {
	resourceInfo()
}

type DCInfo struct {
	ConfigPath string
	YAMLRaw    []byte // for diff'ing when config files change
	DfRaw      []byte // for diff'ing when config files change
}

func (DCInfo) resourceInfo() {}
