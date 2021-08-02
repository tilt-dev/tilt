package portforward

import (
	"fmt"
	"testing"

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
