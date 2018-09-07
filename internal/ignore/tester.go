package ignore

type Tester interface {
	IsIgnored(f string, isDir bool) (bool, error)
}
