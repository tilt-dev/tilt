package state

type Event interface {
	event()
}

type ResourcesEvent struct {
	Resources Resources
}

type SpanEvent struct {
	S Span
}

func (ResourcesEvent) event() {}
func (SpanEvent) event()      {}

type StateReader interface {
	// Ch is a channel of all the new events
	Ch() chan []Event

	// TODO(dbentley): a strictly streaming UI wouldn't need the below,
	// but a reactive one would

	// Get a Span's current state
	GetSpan(id SpanID) (Span, error)

	// Get Children of a Span
	GetChildren(id SpanID) ([]Span, error)
}
