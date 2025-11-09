// Package prompt provides utilities to prompt the user for input.

package prompt

import (
	"bufio"
	"context"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/docker/cli/cli/streams"
	"github.com/moby/term"
)

const ErrTerminated cancelledErr = "prompt terminated"

type cancelledErr string

func (e cancelledErr) Error() string {
	return string(e)
}

func (cancelledErr) Cancelled() {}

// DisableInputEcho disables input echo on the provided streams.In.
// This is useful when the user provides sensitive information like passwords.
// The function returns a restore function that should be called to restore the
// terminal state.
//
// TODO(thaJeztah): implement without depending on streams?
func DisableInputEcho(ins *streams.In) (restore func() error, _ error) {
	oldState, err := term.SaveState(ins.FD())
	if err != nil {
		return nil, err
	}
	restore = func() error {
		return term.RestoreTerminal(ins.FD(), oldState)
	}
	return restore, term.DisableEcho(ins.FD(), oldState)
}

// ReadInput requests input from the user.
//
// It returns an empty string ("") with an [ErrTerminated] if the user terminates
// the CLI with SIGINT or SIGTERM while the prompt is active. If the prompt
// returns an error, the caller should close the [io.Reader] used for the prompt
// and propagate the error up the stack to prevent the background goroutine
// from blocking indefinitely.
func ReadInput(ctx context.Context, in io.Reader, out io.Writer, message string) (string, error) {
	_, _ = out.Write([]byte(message))

	result := make(chan string)
	go func() {
		scanner := bufio.NewScanner(in)
		if scanner.Scan() {
			result <- strings.TrimSpace(scanner.Text())
		}
	}()

	select {
	case <-ctx.Done():
		_, _ = out.Write([]byte("\n"))
		return "", ErrTerminated
	case r := <-result:
		return r, nil
	}
}

// Confirm requests and checks confirmation from the user.
//
// It displays the provided message followed by "[y/N]". If the user
// input 'y' or 'Y' it returns true otherwise false. If no message is provided,
// "Are you sure you want to proceed? [y/N] " will be used instead.
//
// It returns false with an [ErrTerminated] if the user terminates
// the CLI with SIGINT or SIGTERM while the prompt is active. If the prompt
// returns an error, the caller should close the [io.Reader] used for the prompt
// and propagate the error up the stack to prevent the background goroutine
// from blocking indefinitely.
func Confirm(ctx context.Context, in io.Reader, out io.Writer, message string) (bool, error) {
	if message == "" {
		message = "Are you sure you want to proceed?"
	}
	message += " [y/N] "

	_, _ = out.Write([]byte(message))

	// On Windows, force the use of the regular OS stdin stream.
	if runtime.GOOS == "windows" {
		in = streams.NewIn(os.Stdin)
	}

	result := make(chan bool)

	go func() {
		var res bool
		scanner := bufio.NewScanner(in)
		if scanner.Scan() {
			answer := strings.TrimSpace(scanner.Text())
			if strings.EqualFold(answer, "y") {
				res = true
			}
		}
		result <- res
	}()

	select {
	case <-ctx.Done():
		_, _ = out.Write([]byte("\n"))
		return false, ErrTerminated
	case r := <-result:
		return r, nil
	}
}
