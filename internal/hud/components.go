package hud

import (
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/rty"
)

const resourcesScollerName = "resources"
const logScrollerName = "log modal"
const alertScrollerName = "alert"

func (h *Hud) activeScroller() scroller {
	am := h.activeModal()
	if am != nil {
		return am
	} else {
		return h.r.rty.ElementScroller(resourcesScollerName)
	}
}

func (h *Hud) activeModal() modal {
	if h.currentViewState.AlertMessage != "" {
		return makeAlertModal(h.r.rty)
	} else if h.currentViewState.LogModal.TiltLog == view.TiltLogFullScreen ||
		h.currentViewState.LogModal.ResourceLogNumber != 0 {
		return makeLogModal(h.r.rty)
	} else {
		return nil
	}
}

type scroller interface {
	Up()
	Down()
	Top()
	Bottom()
}

type modal interface {
	Up()
	Down()
	Top()
	Bottom()
	Close(vs *view.ViewState)
}

type alertModal struct {
	rty.TextScroller
}

var _ modal = alertModal{}

func makeAlertModal(r rty.RTY) modal {
	return alertModal{r.TextScroller(alertScrollerName)}
}

func (am alertModal) Close(vs *view.ViewState) {
	vs.AlertMessage = ""
}

type logModal struct {
	rty.TextScroller
}

var _ modal = logModal{}

func makeLogModal(r rty.RTY) modal {
	return logModal{r.TextScroller(logScrollerName)}
}

func (lm logModal) Close(vs *view.ViewState) {
	vs.LogModal = view.LogModal{}
}
