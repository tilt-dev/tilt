package prompt

import (
	"context"
	"fmt"

	"github.com/fatih/color"
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

func (p *TerminalPrompt) tiltBuild(st store.RStore) model.TiltBuild {
	state := st.RLockState()
	defer st.RUnlockState()
	return state.TiltBuildInfo
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

	build := p.tiltBuild(st)
	hasBrowserUI := !p.url.Empty()
	serverStatus := "(without browser UI)"
	if hasBrowserUI {
		if p.host == "0.0.0.0" {
			serverStatus = fmt.Sprintf("on %s (listening on 0.0.0.0)", p.url)
		} else {
			serverStatus = fmt.Sprintf("on %s", p.url)
		}
	}

	firstLine := color.GreenString(fmt.Sprintf("Tilt started %s", serverStatus))
	_, _ = fmt.Fprintf(p.stdout, "%s\n", firstLine)
	_, _ = fmt.Fprintf(p.stdout, "%s\n\n", build.HumanBuildStamp())

	if hasBrowserUI {
		_, _ = fmt.Fprintf(p.stdout, "(space) to open the browser\n")
	}

	_, _ = fmt.Fprintf(p.stdout, "(s) to stream logs\n")
	_, _ = fmt.Fprintf(p.stdout, "(h) to open terminal HUD\n")
	_, _ = fmt.Fprintf(p.stdout, "(ctrl-c) to exit\n")

	p.printed = true

	t, err := p.openInput()
	if err != nil {
		st.Dispatch(store.ErrorAction{Error: err})
		return
	}

	keyCh := make(chan runeMessage)

	// One goroutine just pulls input from TTY.
	go func() {
		for ctx.Err() == nil {
			r, err := t.ReadRune()
			if err != nil {
				st.Dispatch(store.ErrorAction{Error: err})
				return
			}

			msg := runeMessage{
				rune:   r,
				stopCh: make(chan bool),
			}
			keyCh <- msg

			close := <-msg.stopCh
			if close {
				break
			}
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
			case msg, ok := <-keyCh:
				if !ok {
					return
				}

				r := msg.rune
				switch r {
				case 's':
					st.Dispatch(SwitchTerminalModeAction{Mode: store.TerminalModeStream})
					msg.stopCh <- true

				case 'h':
					st.Dispatch(SwitchTerminalModeAction{Mode: store.TerminalModeHUD})
					msg.stopCh <- true

				case ' ':
					_, _ = fmt.Fprintf(p.stdout, "Opening browser: %s\n", p.url.String())
					err := p.openURL(p.url.String())
					if err != nil {
						_, _ = fmt.Fprintf(p.stdout, "Error: %v\n", err)
					}
					msg.stopCh <- false
				default:
					_, _ = fmt.Fprintf(p.stdout, "Unrecognized option: %s\n", string(r))
					msg.stopCh <- false

				}
			}
		}
	}()
}

type runeMessage struct {
	rune rune

	// The receiver of this message should
	// ACK the channel when they're done.
	//
	// Sending 'true' indicates that we're switching to a different mode and the
	// input goroutine should stop reading TTY input.
	stopCh chan bool
}

var _ store.Subscriber = &TerminalPrompt{}
