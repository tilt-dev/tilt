# Engine Design Doc
Engine is primarily a `for` loop that takes inputs from a variety of sources, updates state, and makes a decision based off of that state. The rough shape of the for loop is as follows:

```go
state := &state{}
for {
    select {
        // sources like local filesystem, kubernetes, ui, etc
        case ev := <- fsCh:
            e.handleFsEvent(ev)
        case ev := <- k8sCh:
            e.handleK8sEvent(ev)
    }
    // decide what to do: start a pipeline, stop a pipeline
    actions := handle(state)
    // tell subscribers what we took
    updateSubscribers(actions, state.copy())
}
```

When state changes, and only when state changes, can we make decisions about what to do. Only after actions have been taken do we tell subscribers.

## Rules
* No blocking I/O in the for loop
* No long operations in the for loop
* Actions taken in `handle` shouldn’t directly send to channels that this `for`  `select`s on.
