package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"golang.org/x/term"
)

type generateTiltfileResult string

const (
	generateTiltfileError          = "error"
	generateTiltfileUserExit       = "exit"
	generateTiltfileNonInteractive = "non_interactive"
	generateTiltfileAlreadyExists  = "already_exists"
	generateTiltfileCreated        = "created"
	generateTiltfileDeclined       = "declined"
)

// maybeGenerateTiltfile offers to create a Tiltfile if one does not exist and
// the user is running in an interactive (TTY) session.
//
// The generateTiltfileResult value is ALWAYS set, even on error.
func maybeGenerateTiltfile(tfPath string) (generateTiltfileResult, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return generateTiltfileNonInteractive, nil
	}

	if absPath, err := filepath.Abs(tfPath); err != nil {
		// the absolute path is really just to improve CLI output, if the path
		// itself is so invalid this fails, we'll catch + report it via the
		// logic to determine if a Tiltfile already exists
		tfPath = absPath
	}

	if hasTiltfile, err := checkTiltfileExists(tfPath); err != nil {
		// either Tiltfile path is totally invalid or there's something like
		// a permissions error, so report it & exit
		return generateTiltfileError, err
	} else if hasTiltfile {
		// Tiltfile exists, so don't prompt to generate one
		return generateTiltfileAlreadyExists, nil
	}

	t, restoreTerm, err := setupTerm(os.Stdin)
	if err != nil {
		return generateTiltfileError, nil
	}

	lineCount := 0
	var postFinishMessage string
	defer func() {
		_ = restoreTerm()

		if postFinishMessage != "" {
			// NOTE: pre-win10, there's no support for ANSI escape codes, and
			// 	it's not worth the headache to deal with Windows console API
			// 	for this, so the output isn't cleared there
			if err == nil && runtime.GOOS != "windows" {
				// erase our output once done on success
				// \033[%d -> move cursor up %d rows
				// \r      -> move cursor to first column
				// \033[J  -> clear output from cursor to end of stream
				fmt.Printf("\033[%dA\r\033[J", lineCount)
			}

			fmt.Println(postFinishMessage)
		}
	}()

	// Offer to create a Tiltfile
	var intro bytes.Buffer
	intro.WriteString(tiltfileDoesNotExistWarning(tfPath))
	intro.WriteString(`
Tilt can create a sample Tiltfile for you, which includes
useful snippets to modify and extend with build and deploy
steps for your microservices.
`)
	intro.WriteString("\n")
	_, err = t.Write(intro.Bytes())
	if err != nil {
		return generateTiltfileError, err
	}
	// we track # of lines written to clear the output when done
	lineCount += bytes.Count(intro.Bytes(), []byte("\n"))

	for {
		line, err := t.ReadLine()
		lineCount++
		if err != nil {
			// perform a carriage return to ensure we're back at the beginning
			// of a new line (if user hit Ctrl-C/Ctrl-D, this is necessary; for
			// any other errors, better to be safe than leave terminal in a bad
			// state)
			fmt.Println("\r")
			if err == io.EOF {
				// since we have the terminal in raw mode, no signal will be fired
				// on Ctrl-C, so we manually propagate it here (sending ourselves
				// a SIGINT signal is not practical)
				return generateTiltfileUserExit, userExitError
			}
			return generateTiltfileError, err
		}

		line = strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(line, "y") {
			break
		} else if strings.HasPrefix(line, "n") {
			// there's a noticeable delay before further output indicating that
			// Tilt has started, so we don't want users to think Tilt is hung
			postFinishMessage = "Starting Tilt...\n"
			return generateTiltfileDeclined, nil
		}
	}

	if err = os.WriteFile(tfPath, starterTiltfile, 0644); err != nil {
		return generateTiltfileError, fmt.Errorf("could not write to %s: %v", tfPath, err)
	}

	postFinishMessage = generateTiltfileSuccessMessage(tfPath)
	return generateTiltfileCreated, nil
}

func setupTerm(f *os.File) (t *term.Terminal, restore func() error, err error) {
	oldState, err := term.MakeRaw(int(f.Fd()))
	if err != nil {
		return nil, nil, err
	}
	restore = func() error {
		return term.Restore(int(f.Fd()), oldState)
	}
	t = term.NewTerminal(os.Stdin, "✨ Create a starter Tiltfile? (y/n) ")
	return t, restore, nil
}

func checkTiltfileExists(tfPath string) (bool, error) {
	if fi, err := os.Stat(tfPath); err == nil {
		if fi.Mode().IsDir() {
			return false, fmt.Errorf("could not open Tiltfile at %s: target is a directory", tfPath)
		}
		// Tiltfile exists!
		return true, nil
	} else if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else {
		// likely a permissions issue, bubble up the error and exit Tilt
		// N.B. os::Stat always returns a PathError which will include the path in its output
		return false, fmt.Errorf("could not open Tiltfile: %v", err)
	}
}

func tiltfileDoesNotExistWarning(tfPath string) string {
	return color.YellowString(`
───────────────────────────────────────────────────────────
 ⚠️  No Tiltfile exists at %s
───────────────────────────────────────────────────────────
`, tfPath)
}

func generateTiltfileSuccessMessage(tfPath string) string {
	return fmt.Sprintf(`
───────────────────────────────────────────────────────────
 🎉  Tiltfile generated at %s
───────────────────────────────────────────────────────────
`, color.BlueString(tfPath))
}
