package prompt

import (
	"context"
	"fmt"

	tty "github.com/mattn/go-tty"
	"github.com/pkg/browser"

	"github.com/tilt-dev/tilt/internal/hud"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

type TerminalInput interface {
	ReadRune() (rune, error)
	Close() error
}

type OpenInput func() (TerminalInput, error)

type OpenURL func(url string) error

func TTYOpen() (TerminalInput, error) {
	return tty.Open()
}

func BrowserOpen(url string) error {
	return browser.OpenURL(url)
}

type TerminalPrompt struct {
	openInput OpenInput
	openURL   OpenURL
	stdout    hud.Stdout
	host      model.WebHost
	url       model.WebURL
	printed   bool
}

func NewTerminalPrompt(openInput OpenInput, openURL OpenURL, stdout hud.Stdout, host model.WebHost, url model.WebURL) *TerminalPrompt {
	return &TerminalPrompt{
		openInput: openInput,
		openURL:   openURL,
		stdout:    stdout,
		host:      host,
		url:       url,
	}
}

func (p *TerminalPrompt) isEnabled(st store.RStore) bool {
	state := st.RLockState()
	defer st.RUnlockState()
	return state.TerminalMode == store.TerminalModePrompt
}

func (p *TerminalPrompt) OnChange(ctx context.Context, st store.RStore) {
	if !p.isEnabled(st) {
		return
	}

	if p.printed {
		return
	}

	hasBrowserUI := !p.url.Empty()
	serverStatus := "(without browser UI)"
	if hasBrowserUI {
		if p.host == "0.0.0.0" {
			serverStatus = fmt.Sprintf("on %s (listening on 0.0.0.0)", p.url)
		} else {
			serverStatus = fmt.Sprintf("on %s", p.url)
		}
	}

	_, _ = fmt.Fprintf(p.stdout, "Tilt started %s\n", serverStatus)

	if hasBrowserUI {
		_, _ = fmt.Fprintf(p.stdout, "(space) to open the browser\n")
	}

	// TODO(nick): implement this
	// _, _ = fmt.Fprintf(p.stdout, "(s) to stream logs\n")
	_, _ = fmt.Fprintf(p.stdout, "(ctrl-c) to exit\n")

	p.printed = true

	t, err := p.openInput()
	if err != nil {
		st.Dispatch(store.ErrorAction{Error: err})
		return
	}

	keyCh := make(chan rune)

	// One goroutine just pulls input from TTY.
	go func() {
		for ctx.Err() == nil {
			r, err := t.ReadRune()
			if err != nil {
				st.Dispatch(store.ErrorAction{Error: err})
				return
			}
			keyCh <- r
		}
		close(keyCh)
	}()

	// Another goroutine processes the input. Doing this
	// on a separate goroutine allows us to clean up the TTY
	// even if it's still blocking on the ReadRune
	go func() {
		defer func() {
			_ = t.Close()
		}()

		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				return
			case r, ok := <-keyCh:
				if !ok {
					return
				}

				switch r {
				case ' ':
					_, _ = fmt.Fprintf(p.stdout, "Opening browser: %s\n", p.url.String())
					err := p.openURL(p.url.String())
					if err != nil {
						_, _ = fmt.Fprintf(p.stdout, "Error: %v\n", err)
					}
				default:
					_, _ = fmt.Fprintf(p.stdout, "Unrecognized option: %s\n", string(r))

				}
			}
		}
	}()
}

var _ store.Subscriber = &TerminalPrompt{}
