[![GoDoc](https://godoc.org/github.com/wojas/genericr?status.svg)](https://godoc.org/github.com/wojas/genericr)

# genericr

A generic [logr](https://github.com/go-logr/logr) implementation that makes it easy
to write your own logging backend.

This package allows you to implement the logr interface by implementing a single
callback function instead of the full interface. 

## Examples

### Basic example

Basic example that directly writes to stderr:

```go
log := genericr.New(func(e genericr.Entry) {
	fmt.Fprintln(os.Stderr, e.String())
})

log.WithName("some-component").WithValues("x", 123).V(1).Info("some event", "y", 42)
```

This outputs the default string representation which currently looks like this:

```
[1] some-component "some event" x=123 y=42
```

The `genericr.Entry` struct you receive in your handler contains the following fields
that you can use to do your own formatting:

```go
type Entry struct {
	Level     int           // level at which this was logged
	Name      string        // name parts joined with '.'
	NameParts []string      // individual name segments
	Message   string        // message as send to log call
	Error     error         // error if .Error() was called
	Fields    []interface{} // alternating key-value pairs

	// Caller information
	Caller      runtime.Frame // only available after .WithCaller(true)
	CallerDepth int           // caller depth from callback
}
```

Use `e.FieldsMap()` to retrieve the Fields as a map.

To enable caller information, use `genericr.New(...).WithCaller(true)`.

To filter messages above a certain verbosity level, use `genericr.New(...).WithVerbosity(1)`.

### Usage in tests

This shows how you can redirect logs to the testing.T logger and keep a reference
to the last log entry if you want to check that one:

```go
func TestLogger_Table(t *testing.T) {
	var last genericr.Entry
	log := genericr.New(func(e genericr.Entry) {
		last = e
		t.Log(e.String())
	})
    // ...
}
```

### Logrus example

More complicated example that implements a [logrus](https://github.com/sirupsen/logrus) adapter:

```go
root := logrus.New()
root.SetLevel(logrus.TraceLevel)
root.SetFormatter(&logrus.TextFormatter{
	DisableColors:    true,
	DisableTimestamp: true,
})
root.Out = os.Stdout

var lf genericr.LogFunc = func(e genericr.Entry) {
	var l *logrus.Entry = root.WithField("component", e.Name)
	if e.Error != nil {
		l = l.WithError(e.Error)
	}
	if len(e.Fields) != 0 {
		l = l.WithFields(e.FieldsMap())
	}
	logrusLevel := logrus.Level(e.Level) + logrus.InfoLevel // 0 = info
	if logrusLevel < logrus.ErrorLevel {
		logrusLevel = logrus.ErrorLevel
	} else if logrusLevel > logrus.TraceLevel {
		logrusLevel = logrus.TraceLevel
	}
	l.Log(logrusLevel, e.Message)
}
log := genericr.New(lf)
```

