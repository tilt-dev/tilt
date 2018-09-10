package ignore

// TODO(nick): should we unify this with model.FileMatcher?
type Tester interface {
	IsIgnored(f string, isDir bool) (bool, error)
}
