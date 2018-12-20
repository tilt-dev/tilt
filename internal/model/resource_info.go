package model

type deployInfo interface {
	deployInfo()
}

type DCInfo struct {
	ConfigPath string
	YAMLRaw    []byte // for diff'ing when config files change
	DfRaw      []byte // for diff'ing when config files change
}

func (DCInfo) deployInfo() {}
