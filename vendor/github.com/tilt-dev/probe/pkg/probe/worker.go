package probe

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"k8s.io/klog/v2"

	"github.com/tilt-dev/probe/pkg/prober"
)

const (
	// DefaultProbePeriod is the default value for the interval between
	// probe invocations.
	DefaultProbePeriod = 10 * time.Second

	// DefaultProbeTimeout is the default value for the timeout when
	// executing a probe to cancel it and consider it failed.
	DefaultProbeTimeout = 1 * time.Second

	// DefaultInitialDelay is the default value for the initial delay
	// before beginning to invoke the probe after the Worker is started.
	DefaultInitialDelay = 0 * time.Second

	// DefaultProbeSuccessThreshold is the default value for the
	// minimum number of consecutive successes required after having
	// failed before the status will transition to probe.Success.
	DefaultProbeSuccessThreshold = 1

	// DefaultProbeFailureThreshold is the default value for the
	// minimum number of consecutive failures required after having
	// succeeded before the status will transition to probe.Failure.
	DefaultProbeFailureThreshold = 3
)

// realClock is a thin wrapper around Go stdlib methods; a global
// instance is shared to avoid allocating for every Worker. It is
// safe to use from multiple Goroutines.
var realClock = clockwork.NewRealClock()

// StatusChangedFunc is invoked on status transitions.
//
// It will NOT be called for subsequent probe invocations that do not
// result in a status change.
type StatusChangedFunc func(status prober.Result, output string)

// WorkerOption can be passed when creating a Worker to configure the
// instance.
type WorkerOption func(w *Worker)

type probeResult struct {
	result prober.Result
	output string
	err    error
}

// NewWorker creates a Worker instance using the provided probe.Prober
// and options (if any).
func NewWorker(p prober.Prober, opts ...WorkerOption) *Worker {
	w := &Worker{
		prober:           p,
		clock:            realClock,
		period:           DefaultProbePeriod,
		timeout:          DefaultProbeTimeout,
		initialDelay:     DefaultInitialDelay,
		successThreshold: DefaultProbeSuccessThreshold,
		failureThreshold: DefaultProbeFailureThreshold,
		status:           prober.Unknown,
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

// Worker handles executing probes and reporting results.
//
// It's loosely based (but simplified) on the k8s.io/kubernetes/pkg/kubelet/prober design.
type Worker struct {
	// probe is the actual logic that will be invoked to determine status.
	prober prober.Prober
	// clock is used to create timers and facilitate easier unit testing.
	clock clockwork.Clock
	// mu guards mutable state that can be accessed from multiple goroutines (see docs on
	// individual fields for which mu must be held before access).
	mu sync.Mutex
	// stopFunc is invoked when a running Worker instance is stopped to cancel the context.
	//
	// mu must be held before accessing.
	stopFunc context.CancelFunc
	// initialDelay is the amount of time before the probe is first executed.
	initialDelay time.Duration
	// period is the interval on which the probe is executed.
	period time.Duration
	// timeout is the maximum duration for which a probe can execute before it's considered
	// to have failed (and its result ignored).
	timeout time.Duration
	// successThreshold is the number of times a probe must succeed after previously having
	// failed before it will transition to a successful state.
	successThreshold int
	// failureThreshold is the number of times a probe must fail after previously having
	// been successful before it will transition to a failure state.
	failureThreshold int
	// resultsChan receives ALL probe execution results, including duplicates.
	//
	// Currently, this is only exposed internally for testing to force synchronization.
	resultsChan chan probeResult
	// status is only updated after the failure/success threshold is crossed.
	//
	// mu must be held before accessing.
	status prober.Result
	// statusFunc is an optional function to call whenever the status changes.
	statusFunc StatusChangedFunc
	// lastResult is the result of the previous probe execution and is used along with
	// resultRun to determine when a threshold has been crossed.
	lastResult prober.Result
	// resultRun is the number of times the probe has returned the same result and is
	// used along with lastResult to determine when a threshold has been crossed.
	resultRun int
}

// Run periodically executes a probe until stopped.
//
// The Worker can be stopped by explicitly calling Stop() or implicitly
// via context cancellation.
//
// Calling Run() on an instance that is already running will result in
// a panic.
func (w *Worker) Run(ctx context.Context) {
	w.mu.Lock()
	if w.stopFunc != nil {
		panic("prober is already running")
	}
	ctx, cancel := context.WithCancel(ctx)
	w.stopFunc = cancel

	w.lastResult = prober.Unknown
	w.resultRun = 0
	// initial status is failure until a successful probe
	w.status = prober.Failure

	w.mu.Unlock()

	w.clock.Sleep(w.initialDelay)

	ticker := w.clock.NewTicker(w.period)
	for {
		w.doProbe(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.Chan():
		}
	}
}

// Stop halts further probe invocations. It is safe to call Stop()
// more than once.
func (w *Worker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopFunc != nil {
		w.stopFunc()
		w.stopFunc = nil
		w.status = prober.Unknown
	}
}

// Status returns the current probe result.
//
// If not running, this will always return probe.Unknown.
func (w *Worker) Status() prober.Result {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.status
}

func (w *Worker) doProbe(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()
	result := make(chan probeResult, 1)
	go func() {
		r, out, err := w.prober.Probe(ctx)
		result <- probeResult{result: r, output: out, err: err}
	}()

	for {
		select {
		case r := <-result:
			w.handleResult(r)
			return
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				// only context deadline exceeded triggers a result handling
				// (if context was explicitly canceled, there's no reason to
				// record a result as the prober is being stopped)
				w.handleResult(probeResult{result: prober.Failure, err: ctx.Err()})
			}
			return
		}
	}
}

// handleResult updates prober internal state based on the probe result.
//
// This is very similar to https://github.com/kubernetes/kubernetes/blob/v1.20.2/pkg/kubelet/prober/worker.go#L260-L273
func (w *Worker) handleResult(probeResult probeResult) {
	if w.resultsChan != nil {
		defer func() {
			w.resultsChan <- probeResult
		}()
	}

	result := probeResult.result
	if probeResult.err != nil {
		if !errors.Is(probeResult.err, context.DeadlineExceeded) {
			// the probe itself returned an error, so ignore this execution
			klog.V(4).ErrorS(probeResult.err, "Probe returned an error; result ignored")
			return
		}
		result = prober.Failure
	}

	if w.lastResult == result {
		w.resultRun++
	} else {
		w.lastResult = result
		w.resultRun = 1
	}

	success := isSuccessResult(result)
	if (!success && w.resultRun < w.failureThreshold) ||
		(success && w.resultRun < w.successThreshold) {
		return
	}

	w.mu.Lock()
	if w.stopFunc == nil || w.status == result {
		w.mu.Unlock()
		return
	}
	w.status = result
	w.mu.Unlock()

	if w.statusFunc != nil {
		w.statusFunc(result, probeResult.output)
	}
}

// isSuccessResult coerces a probe.Result value into a bool based on
// whether it's considered a successful value or not.
func isSuccessResult(result prober.Result) bool {
	if result == prober.Success || result == prober.Warning {
		return true
	}
	return false
}

// WorkerPeriod sets the period between probe invocations.
func WorkerPeriod(period time.Duration) WorkerOption {
	return func(w *Worker) {
		w.period = period
	}
}

// WorkerTimeout sets the duration before a running probe is canceled
// and considered to have failed.
func WorkerTimeout(timeout time.Duration) WorkerOption {
	return func(w *Worker) {
		w.timeout = timeout
	}
}

// WorkerFailureThreshold sets the number of consecutive failures
// required after a probe has succeeded before the status will
// transition to probe.Failure.
func WorkerFailureThreshold(v int) WorkerOption {
	return func(w *Worker) {
		w.failureThreshold = v
	}
}

// WorkerSuccessThreshold sets the number of consecutive successes
// required after a probe has failed before the status will
// transition to probe.Success.
func WorkerSuccessThreshold(v int) WorkerOption {
	return func(w *Worker) {
		w.successThreshold = v
	}
}

// WorkerInitialDelay sets the amount of time that will be waited
// when the prober starts before beginning to invoke the probe.
//
// The status will be probe.Failure during the initial delay
// period.
func WorkerInitialDelay(delay time.Duration) WorkerOption {
	return func(w *Worker) {
		w.initialDelay = delay
	}
}

// WorkerOnStatusChange sets the function to invoke when the status
// transitions.
//
// Subsequent probe invocations that do not result in a change to the
// status (either because they return the same result or the failure/
// success threshold has not been met) will not emit a status change
// update.
func WorkerOnStatusChange(f StatusChangedFunc) WorkerOption {
	return func(w *Worker) {
		w.statusFunc = f
	}
}
