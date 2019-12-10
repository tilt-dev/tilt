package hud

import (
	"fmt"

	"github.com/gdamore/tcell"

	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/rty"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/model/logstore"
)

type TabView struct {
	view      view.View
	viewState view.ViewState
	tabState  view.TabState
}

func NewTabView(v view.View, vState view.ViewState) *TabView {
	return &TabView{
		view:      v,
		viewState: vState,
		tabState:  vState.TabState,
	}
}

func (v *TabView) Build() rty.Component {
	l := rty.NewConcatLayout(rty.DirVert)
	l.Add(v.buildTabs(false))

	log := rty.NewTextScrollLayout("log")
	log.Add(rty.TextString(v.log()))
	l.Add(log)

	return l
}

func (v *TabView) log() string {
	var ret model.Log
	var reader logstore.Reader
	switch v.tabState {
	case view.TabAllLog:
		reader = v.view.LogReader
	case view.TabBuildLog:
		_, resource := selectedResource(v.view, v.viewState)
		if !resource.CurrentBuild.Empty() {
			ret = resource.CurrentBuild.Log
		} else {
			ret = resource.LastBuild().Log
		}
	case view.TabPodLog:
		_, resource := selectedResource(v.view, v.viewState)
		if resource.ResourceInfo != nil {
			ret = resource.ResourceInfo.RuntimeLog()
		}
	}

	if !ret.Empty() {
		return ret.Tail(logLineCount).String()
	} else if !reader.Empty() {
		return reader.String()
	} else {
		return "(no logs received)"
	}
}

func (v *TabView) buildTab(text string) rty.Component {
	return rty.TextString(fmt.Sprintf(" %s ", text))
}

func (v *TabView) buildTabs(isMax bool) rty.Component {
	l := rty.NewLine()
	if v.tabState == view.TabAllLog {
		l.Add(v.buildTab("1: ALL LOGS"))
	} else {
		l.Add(v.buildTab("1: all logs"))
	}
	l.Add(rty.TextString("│"))
	if v.tabState == view.TabBuildLog {
		l.Add(v.buildTab("2: BUILD LOG"))
	} else {
		l.Add(v.buildTab("2: build log"))
	}
	l.Add(rty.TextString("│"))
	if v.tabState == view.TabPodLog {
		l.Add(v.buildTab("3: POD LOG"))
	} else {
		l.Add(v.buildTab("3: pod log"))
	}
	l.Add(rty.TextString("│ "))
	l.Add(renderPaneHeader(isMax))
	result := rty.Bg(l, tcell.ColorWhiteSmoke)
	result = rty.Fg(result, cText)
	return result
}
