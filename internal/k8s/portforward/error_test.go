package portforward

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestManyErrors(t *testing.T) {
	h := newErrorHandler()
	defer h.Close()

	go func() {
		h.Stop(fmt.Errorf("my error 1"))
	}()
	go func() {
		h.Stop(fmt.Errorf("my error 2"))
	}()
	go func() {
		h.Stop(fmt.Errorf("my error 3"))
	}()

	err := <-h.Done()
	assert.Contains(t, err.Error(), "my error")
}

func TestNoErrors(t *testing.T) {
	h := newErrorHandler()
	go func() {
		h.Close()
	}()

	err := <-h.Done()
	assert.Nil(t, err)
}

func TestStopWithoutReaderDoesNotBlock(t *testing.T) {
	h := newErrorHandler()
	stopDone := make(chan struct{})
	go func() {
		h.Stop(fmt.Errorf("unobserved error"))
		close(stopDone)
	}()

	select {
	case <-stopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop blocked sending an error when there was no reader")
	}

	// A queued first error remains readable even after Close.
	h.Close()
	assert.Contains(t, (<-h.Done()).Error(), "unobserved error")
}

func TestConcurrentStopCloseWithoutReader(t *testing.T) {
	h := newErrorHandler()
	start := make(chan struct{})
	stopDone := make(chan struct{})
	closeDone := make(chan struct{})
	go func() {
		<-start
		h.Stop(fmt.Errorf("concurrent error"))
		close(stopDone)
	}()
	go func() {
		<-start
		h.Close()
		close(closeDone)
	}()
	close(start)

	select {
	case <-stopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop remained blocked while Close waited for its once-protected send")
	}
	select {
	case <-closeDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not finish after concurrent Stop")
	}
	if err := <-h.Done(); err != nil {
		assert.Contains(t, err.Error(), "concurrent error")
	}
}
