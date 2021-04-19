package prompt

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/fatih/color"
	tty "github.com/mattn/go-tty"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/hud"
	"github.com/tilt-dev/tilt/internal/openurl"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

type TerminalInput interface {
	ReadRune() (rune, error)
	Close() error
}

type OpenInput func() (TerminalInput, error)

func TTYOpen() (TerminalInput, error) {
	return tty.Open()
}

type TerminalPrompt struct {
	a         *analytics.TiltAnalytics
	openInput OpenInput
	openURL   openurl.OpenURL
	stdout    hud.Stdout
	host      model.WebHost
	url       model.WebURL

	printed bool
	term    TerminalInput

	// Make sure that Close() completes both during the teardown sequence and when
	// we switch modes.
	closeOnce sync.Once

	initOutput *bytes.Buffer
}

func NewTerminalPrompt(a *analytics.TiltAnalytics, openInput OpenInput,
	openURL openurl.OpenURL, stdout hud.Stdout,
	host model.WebHost, url model.WebURL) *TerminalPrompt {

	return &TerminalPrompt{
		a:         a,
		openInput: openInput,
		openURL:   openURL,
		stdout:    stdout,
		host:      host,
		url:       url,
	}
}

// Copy initial warnings and info logs from the logstore into the terminal
// prompt, so that they get shown as part of the prompt.
//
// This sits at the intersection of two incompatible interfaces:
//
// 1) The LogStore is an asynchronous, streaming log interface that makes sure
//    all logs are shown everywhere (across stdout, hud, web, snapshots, etc).
//
// 2) The TerminalPrompt is a synchronous interface that shows a deliberately
//    short "greeting" message, then blocks on user input.
//
// Rather than make these two interfaces interoperate well, we just have
// the internal/cli code copy over the logs during the init sequence.
// It's OK if logs show up twice.
func (p *TerminalPrompt) SetInitOutput(buf *bytes.Buffer) {
	p.initOutput = buf
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

func (p *TerminalPrompt) TearDown(ctx context.Context) {
	if p.term != nil {
		p.closeOnce.Do(func() {
			_ = p.term.Close()
		})
	}
}

func (p *TerminalPrompt) OnChange(ctx context.Context, st store.RStore, _ store.ChangeSummary) {
	if !p.isEnabled(st) {
		return
	}

	if p.printed {
		return
	}

	build := p.tiltBuild(st)
	buildStamp := build.HumanBuildStamp()
	firstLine := StartStatusLine(p.url, p.host)
	_, _ = fmt.Fprintf(p.stdout, "%s\n", firstLine)
	_, _ = fmt.Fprintf(p.stdout, "%s\n\n", buildStamp)

	// Print all the init output. See comments on SetInitOutput()
	infoLines := strings.Split(strings.TrimRight(p.initOutput.String(), "\n"), "\n")
	needsNewline := false
	for _, line := range infoLines {
		if strings.HasPrefix(line, firstLine) || strings.HasPrefix(line, buildStamp) {
			continue
		}
		_, _ = fmt.Fprintf(p.stdout, "%s\n", line)
		needsNewline = true
	}

	if needsNewline {
		_, _ = fmt.Fprintf(p.stdout, "\n")
	}

	hasBrowserUI := !p.url.Empty()
	if hasBrowserUI {
		_, _ = fmt.Fprintf(p.stdout, "(space) to open the browser\n")
	}

	_, _ = fmt.Fprintf(p.stdout, "(s) to stream logs (--stream=true)\n")
	_, _ = fmt.Fprintf(p.stdout, "(t) to open legacy terminal mode (--legacy=true)\n")
	_, _ = fmt.Fprintf(p.stdout, "(ctrl-c) to exit\n")

	p.printed = true

	t, err := p.openInput()
	if err != nil {
		st.Dispatch(store.ErrorAction{Error: err})
		return
	}
	p.term = t

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
			p.closeOnce.Do(func() {
				_ = p.term.Close()
			})
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
					p.a.Incr("ui.prompt.switch", map[string]string{"type": "stream"})
					st.Dispatch(SwitchTerminalModeAction{Mode: store.TerminalModeStream})
					msg.stopCh <- true

				case 't', 'h':
					p.a.Incr("ui.prompt.switch", map[string]string{"type": "hud"})
					st.Dispatch(SwitchTerminalModeAction{Mode: store.TerminalModeHUD})

					msg.stopCh <- true

				case ' ':
					p.a.Incr("ui.prompt.browser", map[string]string{})
					_, _ = fmt.Fprintf(p.stdout, "Opening browser: %s\n", p.url.String())
					err := p.openURL(p.url.String(), p.stdout)
					if err != nil {
						_, _ = fmt.Fprintf(p.stdout, "Error: %v\n", err)
					}
					msg.stopCh <- false
				default:
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

func StartStatusLine(url model.WebURL, host model.WebHost) string {
	hasBrowserUI := !url.Empty()
	serverStatus := "(without browser UI)"
	if hasBrowserUI {
		if host == "0.0.0.0" {
			serverStatus = fmt.Sprintf("on %s (listening on 0.0.0.0)", url)
		} else {
			serverStatus = fmt.Sprintf("on %s", url)
		}
	}

	return color.GreenString(fmt.Sprintf("Tilt started %s", serverStatus))
}

var _ store.Subscriber = &TerminalPrompt{}
var _ store.TearDowner = &TerminalPrompt{}
