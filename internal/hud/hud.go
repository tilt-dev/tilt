package hud

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/browser"
	"github.com/pkg/errors"
	"github.com/windmilleng/tcell"

	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/store"
)

// The main loop ensures the HUD updates at least this often
const DefaultRefreshInterval = 100 * time.Millisecond

// number of arrows a pgup/dn is equivalent to
// (we don't currently worry about trying to know how big a page is, and instead just support pgup/dn as "faster arrows"
const pgUpDownCount = 20

type HeadsUpDisplay interface {
	store.Subscriber

	Run(ctx context.Context, dispatch func(action store.Action), refreshRate time.Duration) error
	Update(v view.View, vs view.ViewState) error
	Close()
	SetNarrationMessage(ctx context.Context, msg string) error
}

type Hud struct {
	r *Renderer

	currentView      view.View
	currentViewState view.ViewState
	mu               sync.RWMutex
	isRunning        bool
}

var _ HeadsUpDisplay = (*Hud)(nil)

func NewDefaultHeadsUpDisplay(renderer *Renderer) (HeadsUpDisplay, error) {
	return &Hud{
		r: renderer,
	}, nil
}

func (h *Hud) SetNarrationMessage(ctx context.Context, msg string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	currentViewState := h.currentViewState
	currentViewState.ShowNarration = true
	currentViewState.NarrationMessage = msg
	return h.setViewState(ctx, currentViewState)
}

func (h *Hud) Run(ctx context.Context, dispatch func(action store.Action), refreshRate time.Duration) error {
	h.mu.Lock()
	h.isRunning = true
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		h.isRunning = false
		h.mu.Unlock()
	}()

	screenEvents, err := h.r.SetUp()
	if err != nil {
		return errors.Wrap(err, "error initializing renderer")
	}

	defer h.Close()

	if refreshRate == 0 {
		refreshRate = DefaultRefreshInterval
	}
	ticker := time.NewTicker(refreshRate)

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			if err != context.Canceled {
				return err
			} else {
				return nil
			}
		case e := <-screenEvents:
			done := h.handleScreenEvent(ctx, dispatch, e)
			if done {
				return nil
			}
		case <-ticker.C:
			err := h.Refresh(ctx)
			if err != nil {
				return err
			}
		}
	}
}

func (h *Hud) Close() {
	h.r.Reset()
}

func (h *Hud) handleScreenEvent(ctx context.Context, dispatch func(action store.Action), ev tcell.Event) (done bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	escape := func() bool {
		am := h.activeModal()
		if am != nil {
			am.Close(&h.currentViewState)
			return false
		}

		h.Close()
		dispatch(NewExitAction(nil))
		return true
	}

	switch ev := ev.(type) {
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyEscape:
			if escape() {
				return true
			}

		case tcell.KeyRune:
			switch r := ev.Rune(); {
			case r == 'b': // [B]rowser
				// If we have an endpoint(s), open the first one
				// TODO(nick): We might need some hints on what load balancer to
				// open if we have multiple, or what path to default to on the opened manifest.
				_, selected := h.selectedResource()
				if len(selected.Endpoints) > 0 {
					err := browser.OpenURL(selected.Endpoints[0])
					if err != nil {
						h.currentViewState.AlertMessage = fmt.Sprintf("error opening url '%s' for resource '%s': %v",
							selected.Endpoints[0], selected.Name, err)
					}
				} else {
					h.currentViewState.AlertMessage = fmt.Sprintf("no urls for resource '%s' ¯\\_(ツ)_/¯", selected.Name)
				}
			case r == 'l': // Tilt [L]og
				am := h.activeModal()
				_, isLogModal := am.(logModal)
				if !isLogModal {
					// Close any existing log modal
					if am != nil {
						am.Close(&h.currentViewState)
					}
				}
				h.currentViewState.CycleViewLogState()
			case r == 'k':
				h.activeScroller().Up()
			case r == 'j':
				h.activeScroller().Down()
			case r == 'q': // [Q]uit
				if escape() {
					return true
				}
			case r == 'R': // hidden key for recovering from printf junk during demos
				h.r.screen.Sync()
			}
		case tcell.KeyUp:
			h.activeScroller().Up()
		case tcell.KeyDown:
			h.activeScroller().Down()
		case tcell.KeyPgUp:
			for i := 0; i < pgUpDownCount; i++ {
				h.activeScroller().Up()
			}
		case tcell.KeyPgDn:
			for i := 0; i < pgUpDownCount; i++ {
				h.activeScroller().Down()
			}
		case tcell.KeyEnter:
			if h.activeModal() == nil {
				selectedIdx, r := h.selectedResource()

				if r.IsYAMLManifest {
					h.currentViewState.AlertMessage = fmt.Sprintf("YAML Resources don't have logs")
					break
				}

				h.currentViewState.LogModal = view.LogModal{ResourceLogNumber: selectedIdx + 1}
				h.activeModal().Bottom()
			}
		case tcell.KeyRight:
			i, _ := h.selectedResource()
			h.currentViewState.Resources[i].CollapseState = view.CollapseNo
		case tcell.KeyLeft:
			i, _ := h.selectedResource()
			h.currentViewState.Resources[i].CollapseState = view.CollapseYes
		case tcell.KeyHome:
			h.activeScroller().Top()
		case tcell.KeyEnd:
			h.activeScroller().Bottom()
		case tcell.KeyCtrlC:
			h.Close()
			dispatch(NewExitAction(nil))
			return true
		}

	case *tcell.EventResize:
		// since we already refresh after the switch, don't need to do anything here
		// just marking this as where sigwinch gets handled
	}

	err := h.refresh(ctx)
	if err != nil {
		dispatch(NewExitAction(err))
	}

	return false
}

func (h *Hud) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	view := store.StateToView(state)
	st.RUnlockState()

	h.mu.Lock()
	defer h.mu.Unlock()
	err := h.setView(ctx, view)
	if err != nil {
		st.Dispatch(NewExitAction(err))
	}
}

func (h *Hud) Refresh(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.refresh(ctx)
}

// Must hold the lock
func (h *Hud) setView(ctx context.Context, view view.View) error {
	h.currentView = view

	// if the hud isn't running, make sure new logs are visible on stdout
	if !h.isRunning && h.currentViewState.ProcessedLogByteCount < len(view.Log) {
		fmt.Print(view.Log[h.currentViewState.ProcessedLogByteCount:])
	}

	h.currentViewState.ProcessedLogByteCount = len(view.Log)

	return h.refresh(ctx)
}

// Must hold the lock
func (h *Hud) setViewState(ctx context.Context, currentViewState view.ViewState) error {
	h.currentViewState = currentViewState
	return h.refresh(ctx)
}

// Must hold the lock
func (h *Hud) refresh(ctx context.Context) error {
	// TODO: We don't handle the order of resources changing
	for len(h.currentViewState.Resources) < len(h.currentView.Resources) {
		h.currentViewState.Resources = append(h.currentViewState.Resources, view.ResourceViewState{})
	}

	vs := h.currentViewState
	for _, r := range h.currentViewState.Resources {
		vs.Resources = append(vs.Resources, r)
	}

	return h.Update(h.currentView, h.currentViewState)
}

func (h *Hud) Update(v view.View, vs view.ViewState) error {
	err := h.r.Render(v, vs)
	return errors.Wrap(err, "error rendering hud")
}

func (h *Hud) selectedResource() (i int, resource view.Resource) {
	i = h.r.rty.ElementScroller("resources").GetSelectedIndex()
	if i >= 0 && i < len(h.currentView.Resources) {
		resource = h.currentView.Resources[i]
	}
	return i, resource
}

var _ store.Subscriber = &Hud{}
