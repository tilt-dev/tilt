import React, { Component } from "react"
import AppController from "./AppController"
import NoMatch from "./NoMatch"
import LoadingScreen from "./LoadingScreen"
import Sidebar, { SidebarItem } from "./Sidebar"
import Statusbar, { StatusItem } from "./Statusbar"
import LogPane from "./LogPane"
import K8sViewPane from "./K8sViewPane"
import PreviewPane from "./PreviewPane"
import { Map } from "immutable"
import { Router, Route, Switch, RouteComponentProps } from "react-router-dom"
import { createBrowserHistory, History, UnregisterCallback } from "history"
import "./HUD.scss"
import { incr } from "./analytics"

type HudProps = {}

type Resource = {
  Name: string
  CombinedLog: string
  BuildHistory: Array<any>
  CrashLog: string
  CurrentBuild: any
  DirectoriesWatched: Array<any>
  Endpoints: Array<string>
  IsTiltfile: boolean
  LastDeployTime: string
  PathsWatched: Array<string>
  PendingBuildEdits: any
  PendingBuildReason: number
  ResourceInfo: {
    PodCreationTime: string
    PodLog: string
    PodName: string
    PodRestarts: number
    PodUpdateStartTime: string
    YAML: string
  }
  RuntimeStatus: string
  ShowBuildStatus: boolean
}

export enum ResourceView {
  Log,
  Preview,
}

type HudState = {
  Message: string
  View: {
    Resources: Array<Resource>
    Log: string
    LogTimestamps: boolean
  } | null
  IsSidebarClosed: boolean
}

// The Main HUD view, as specified in
// https://docs.google.com/document/d/1VNIGfpC4fMfkscboW0bjYYFJl07um_1tsFrbN-Fu3FI/edit#heading=h.l8mmnclsuxl1
class HUD extends Component<HudProps, HudState> {
  private controller: AppController
  private history: History
  private unlisten: UnregisterCallback

  constructor(props: HudProps) {
    super(props)

    this.controller = new AppController(
      `ws://${window.location.host}/ws/view`,
      this
    )

    this.history = createBrowserHistory()
    this.unlisten = () => {}

    this.state = {
      Message: "",
      View: {
        Resources: [],
        Log: "",
        LogTimestamps: false,
      },
      IsSidebarClosed: false,
    }

    this.toggleSidebar = this.toggleSidebar.bind(this)
  }

  componentWillMount() {
    incr("ui.web.init", { ua: window.navigator.userAgent })
    this.unlisten = this.history.listen((location, _) => {
      let isLog =
        location.pathname.startsWith("/r/") &&
        !location.pathname.endsWith("/preview")
      let isPreview = location.pathname.endsWith("/preview")
      let tags = { isLog: isLog.toString(), isPreview: isPreview.toString() }
      incr("ui.web.navigation", tags)
    })
  }

  componentDidMount() {
    this.controller.createNewSocket()
  }

  componentWillUnmount() {
    this.controller.dispose()
    this.unlisten()
  }

  setAppState(state: HudState) {
    this.setState(state)
  }

  toggleSidebar() {
    this.setState(prevState => {
      return Map(prevState)
        .set("IsSidebarClosed", !prevState.IsSidebarClosed)
        .toObject() as HudState // NOTE(dmiller): TypeScript doesn't seem to understand what's going on here so I added a type assertion.
    })
  }

  render() {
    let view = this.state.View
    let message = this.state.Message
    let resources = (view && view.Resources) || []
    if (!resources.length) {
      return <LoadingScreen message={message} />
    }

    let isSidebarClosed = this.state.IsSidebarClosed
    let toggleSidebar = this.toggleSidebar
    let statusItems = resources.map(res => new StatusItem(res))
    let sidebarItems = resources.map(res => new SidebarItem(res))
    let sidebarRoute = (props: RouteComponentProps<any>) => {
      let name = props.match.params.name
      return (
        <Sidebar
          selected={name}
          items={sidebarItems}
          isClosed={isSidebarClosed}
          toggleSidebar={toggleSidebar}
          resourceView={ResourceView.Log}
        />
      )
    }
    let sidebarPreviewRoute = (props: RouteComponentProps<any>) => {
      let name = props.match.params.name
      return (
        <Sidebar
          selected={name}
          items={sidebarItems}
          isClosed={isSidebarClosed}
          toggleSidebar={toggleSidebar}
          resourceView={ResourceView.Preview}
        />
      )
    }

    let logsRoute = (props: RouteComponentProps<any>) => {
      let name = props.match.params ? props.match.params.name : ""
      let logs = ""
      if (view && name !== "") {
        let r = view.Resources.find(r => r.Name === name)
        logs = r ? r.CombinedLog : ""
      }
      return <LogPane log={logs} isExpanded={isSidebarClosed} />
    }

    let combinedLog = ""
    if (view) {
      combinedLog = view.Log
    }

    let previewRoute = (props: RouteComponentProps<any>) => {
      let name = props.match.params ? props.match.params.name : ""
      let endpoint = ""
      if (view && name !== "") {
        let r = view.Resources.find(r => r.Name === name)
        endpoint = r ? r.Endpoints && r.Endpoints[0] : ""
      }

      return <PreviewPane endpoint={endpoint} isExpanded={isSidebarClosed} />
    }

    return (
      <Router history={this.history}>
        <div className="HUD">
          <Switch>
            <Route path="/r/:name/preview" render={sidebarPreviewRoute} />}
            <Route path="/r/:name" render={sidebarRoute} />
            <Route render={sidebarRoute} />
          </Switch>
          <Statusbar items={statusItems} />
          <Switch>
            <Route
              exact
              path="/"
              render={() => (
                <LogPane log={combinedLog} isExpanded={isSidebarClosed} />
              )}
            />
            <Route exact path="/r/:name" render={logsRoute} />
            <Route exact path="/r/:name/k8s" render={() => <K8sViewPane />} />
            <Route exact path="/r/:name/preview" render={previewRoute} />
            <Route component={NoMatch} />
          </Switch>
        </div>
      </Router>
    )
  }
}

export default HUD
