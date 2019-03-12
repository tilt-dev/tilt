package hud

import (
	"fmt"
	"os"

	"github.com/gdamore/tcell"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/rty"
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

func tabsEnabled() bool {
	return os.Getenv("TILT_TAB_VIEW") != ""
}

func (v *TabView) Build() rty.Component {
	if tabsEnabled() {
		return v.buildTabView()
	}
	l := rty.NewConcatLayout(rty.DirHor)
	log := rty.NewTextScrollLayout("log")
	log.Add(rty.TextString(v.view.Log))
	l.Add(log)
	return l
}

func (v *TabView) log() string {
	switch v.tabState {
	case view.TabAllLog:
		return v.view.Log

	case view.TabBuildLog:
		_, resource := selectedResource(v.view, v.viewState)
		if !resource.CurrentBuild.Empty() {
			return resource.CurrentBuild.Log.String()
		}

		return resource.LastBuild().Log.String()

	case view.TabPodLog:
		_, resource := selectedResource(v.view, v.viewState)
		if resource.ResourceInfo == nil {
			return ""
		}
		return resource.ResourceInfo.RuntimeLog()
	}
	return ""
}

func (v *TabView) buildTabView() rty.Component {
	l := rty.NewConcatLayout(rty.DirVert)
	l.Add(v.buildTabs())

	log := rty.NewTextScrollLayout("log")
	log.Add(rty.TextString(v.log()))
	l.Add(log)

	return l
}

func (v *TabView) buildTab(text string) rty.Component {
	return rty.TextString(fmt.Sprintf(" %s ", text))
}

func (v *TabView) buildTabs() rty.Component {
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
	l.Add(renderPaneHeader(v.viewState))
	result := rty.Bg(l, tcell.ColorWhiteSmoke)
	result = rty.Fg(result, cText)
	return result
}
