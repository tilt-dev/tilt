package genericr

// TLogger is the subset of the testing.TB interface we need to log with it
type TLogger interface {
	Log(args ...interface{})
}

// NewForTesting returns a Logger for given testing.T or B.
// Note that the source line reference will be incorrect in all messages
// written by testing.T. There is nothing we can do about that, the call depth
// is hardcoded in there.
func NewForTesting(t TLogger) Logger {
	var f LogFunc = func(e Entry) {
		t.Log(e.String())
	}
	return New(f)
}
