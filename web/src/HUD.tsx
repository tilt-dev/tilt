import React, { Component } from "react"
import AppController from "./AppController"
import NoMatch from "./NoMatch"
import LoadingScreen from "./LoadingScreen"
import Sidebar, { SidebarItem } from "./Sidebar"
import Statusbar, { StatusItem } from "./Statusbar"
import LogPane from "./LogPane"
import K8sViewPane from "./K8sViewPane"
import PreviewPane from "./PreviewPane"
import PathBuilder from "./PathBuilder"
import { Map } from "immutable"
import { Route, Switch, RouteComponentProps } from "react-router-dom"
import { createBrowserHistory, History, UnregisterCallback } from "history"
import { incr, pathToTag } from "./analytics"
import TopBar from "./TopBar"
import "./HUD.scss"
import { TiltBuild, ResourceView } from "./types"
import AlertPane, { AlertResource } from "./AlertPane"
import PreviewList from "./PreviewList"
import AnalyticsNudge from "./AnalyticsNudge"

type HudProps = {
  history: History
}

type Resource = {
  Name: string
  CombinedLog: string
  BuildHistory: Array<any>
  CrashLog: string
  CurrentBuild: any
  DirectoriesWatched: Array<any>
  Endpoints: Array<string>
  PodID: string
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
    PodStatus: string
  }
  RuntimeStatus: string
  ShowBuildStatus: boolean
}

type HudState = {
  Message: string
  View: {
    Resources: Array<Resource>
    Log: string
    LogTimestamps: boolean
    SailEnabled: boolean
    SailURL: string
    NeedsAnalyticsNudge: boolean
    RunningTiltBuild: TiltBuild
    LatestTiltBuild: TiltBuild
  } | null
  IsSidebarClosed: boolean
}

// The Main HUD view, as specified in
// https://docs.google.com/document/d/1VNIGfpC4fMfkscboW0bjYYFJl07um_1tsFrbN-Fu3FI/edit#heading=h.l8mmnclsuxl1
class HUD extends Component<HudProps, HudState> {
  // The root of the HUD view, without the slash.
  private pathBuilder: PathBuilder
  private controller: AppController
  private history: History
  private unlisten: UnregisterCallback

  constructor(props: HudProps) {
    super(props)

    this.pathBuilder = new PathBuilder(
      window.location.host,
      window.location.pathname
    )
    this.controller = new AppController(
      this.pathBuilder.getWebsocketUrl(),
      this
    )
    this.history = props.history
    this.unlisten = () => {}

    this.state = {
      Message: "",
      View: {
        Resources: [],
        Log: "",
        LogTimestamps: false,
        SailEnabled: false,
        SailURL: "",
        NeedsAnalyticsNudge: false,
        RunningTiltBuild: {
          Version: "",
          Date: "",
          Dev: false,
        },
        LatestTiltBuild: {
          Version: "",
          Date: "",
          Dev: false,
        },
      },
      IsSidebarClosed: false,
    }

    this.toggleSidebar = this.toggleSidebar.bind(this)
  }

  componentWillMount() {
    incr("ui.web.init", { ua: window.navigator.userAgent })
    this.unlisten = this.history.listen((location, _) => {
      let tags = { type: pathToTag(location.pathname) }
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

  getPreviewForName(name: string, resources: Array<SidebarItem>): string {
    if (name) {
      return `/r/${name}/preview`
    }

    return `/preview`
  }

  path(relPath: string) {
    return this.pathBuilder.path(relPath)
  }

  render() {
    let view = this.state.View
    let sailEnabled = view && view.SailEnabled ? view.SailEnabled : false
    let sailUrl = view && view.SailURL ? view.SailURL : ""
    let needsNudge = view ? view.NeedsAnalyticsNudge : false
    let message = this.state.Message
    let resources = (view && view.Resources) || []
    if (!resources.length) {
      return <LoadingScreen message={message} />
    }

    let isSidebarClosed = this.state.IsSidebarClosed
    let toggleSidebar = this.toggleSidebar
    let statusItems = resources.map(res => new StatusItem(res))
    let sidebarItems = resources.map(res => new SidebarItem(res))
    let sidebarRoute = (t: ResourceView, props: RouteComponentProps<any>) => {
      let name = props.match.params.name
      return (
        <Sidebar
          selected={name}
          items={sidebarItems}
          isClosed={isSidebarClosed}
          toggleSidebar={toggleSidebar}
          resourceView={t}
          pathBuilder={this.pathBuilder}
        />
      )
    }

    let topBarRoute = (t: ResourceView, props: RouteComponentProps<any>) => {
      let name =
        props.match.params && props.match.params.name
          ? props.match.params.name
          : ""
      let numAlerts = 0
      if (name !== "") {
        let selectedResource = resources.find(r => r.Name === name)
        let er = new AlertResource(selectedResource)
        if (er.hasAlert()) {
          numAlerts = er.numberOfAlerts()
        }
      } else {
        numAlerts = resourcesWithAlerts
          .map(er => er.numberOfAlerts())
          .reduce((sum, current) => sum + current, 0)
      }
      return (
        <TopBar
          logUrl={name === "" ? this.path("/") : this.path(`/r/${name}`)}
          alertsUrl={
            name === "" ? this.path("/alerts") : this.path(`/r/${name}/alerts`)
          }
          previewUrl={this.path(this.getPreviewForName(name, sidebarItems))}
          resourceView={t}
          sailEnabled={sailEnabled}
          sailUrl={sailUrl}
          numberOfAlerts={numAlerts}
        />
      )
    }

    let logsRoute = (props: RouteComponentProps<any>) => {
      let name =
        props.match.params && props.match.params.name
          ? props.match.params.name
          : ""
      let logs = ""
      let endpoints: Array<string> = []
      let podID = ""
      let podStatus = ""
      if (view && name !== "") {
        let r = view.Resources.find(r => r.Name === name)
        logs = (r && r.CombinedLog) || ""
        endpoints = (r && r.Endpoints) || []
        podID = (r && r.PodID) || ""
        podStatus = (r && r.ResourceInfo && r.ResourceInfo.PodStatus) || ""
      }
      return (
        <LogPane
          log={logs}
          isExpanded={isSidebarClosed}
          endpoints={endpoints}
          podID={podID}
          podStatus={podStatus}
        />
      )
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

      if (view && endpoint === "") {
        let resourceNamesWithEndpoints = view.Resources.filter(
          r => r.Endpoints && r.Endpoints.length > 0
        ).map(r => r.Name)
        return (
          <PreviewList
            resourcesWithEndpoints={resourceNamesWithEndpoints}
            pathBuilder={this.pathBuilder}
          />
        )
      }

      return <PreviewPane endpoint={endpoint} isExpanded={isSidebarClosed} />
    }

    let errorRoute = (props: RouteComponentProps<any>) => {
      let name = props.match.params ? props.match.params.name : ""
      let er = resources.find(r => r.Name === name)
      if (er) {
        return <AlertPane resources={[new AlertResource(er)]} />
      }
      return <AlertPane resources={[]} />
    }
    let alertResources = resources.map(r => new AlertResource(r))
    let resourcesWithAlerts = alertResources.filter(r => r.hasAlert())

    let runningVersion = view && view.RunningTiltBuild
    let latestVersion = view && view.LatestTiltBuild

    return (
      <div className="HUD">
        <AnalyticsNudge needsNudge={needsNudge} />
        <Switch>
          <Route
            path={this.path("/r/:name/alerts")}
            render={topBarRoute.bind(null, ResourceView.Alerts)}
          />
          <Route
            path={this.path("/r/:name/preview")}
            render={topBarRoute.bind(null, ResourceView.Preview)}
          />
          <Route
            path={this.path("/preview")}
            render={topBarRoute.bind(null, ResourceView.Preview)}
          />
          <Route
            path={this.path("/r/:name")}
            render={topBarRoute.bind(null, ResourceView.Log)}
          />
          <Route
            path={this.path("/alerts")}
            render={topBarRoute.bind(null, ResourceView.Alerts)}
          />
          <Route render={topBarRoute.bind(null, ResourceView.Log)} />
        </Switch>
        <Switch>
          <Route
            path={this.path("/r/:name/alerts")}
            render={sidebarRoute.bind(null, ResourceView.Alerts)}
          />
          <Route
            path={this.path("/alerts")}
            render={sidebarRoute.bind(null, ResourceView.Alerts)}
          />
          <Route
            path={this.path("/r/:name/preview")}
            render={sidebarRoute.bind(null, ResourceView.Preview)}
          />
          <Route
            path={this.path("/preview")}
            render={sidebarRoute.bind(null, ResourceView.Preview)}
          />
          <Route
            path={this.path("/r/:name")}
            render={sidebarRoute.bind(null, ResourceView.Log)}
          />
          <Route render={sidebarRoute.bind(null, ResourceView.Log)} />
        </Switch>
        <Statusbar
          items={statusItems}
          alertsUrl={this.path("/alerts")}
          runningVersion={runningVersion}
          latestVersion={latestVersion}
        />
        <Switch>
          <Route
            exact
            path={this.path("/")}
            render={() => (
              <LogPane
                log={combinedLog}
                isExpanded={isSidebarClosed}
                podID={""}
                endpoints={[]}
                podStatus={""}
              />
            )}
          />
          <Route
            exact
            path={this.path("/alerts")}
            render={() => <AlertPane resources={alertResources} />}
          />
          <Route exact path={this.path("/preview")} render={previewRoute} />
          <Route exact path={this.path("/r/:name")} render={logsRoute} />
          <Route
            exact
            path={this.path("/r/:name/k8s")}
            render={() => <K8sViewPane />}
          />
          <Route
            exact
            path={this.path("/r/:name/alerts")}
            render={errorRoute}
          />
          <Route
            exact
            path={this.path("/r/:name/preview")}
            render={previewRoute}
          />
          <Route component={NoMatch} />
        </Switch>
      </div>
    )
  }
}

export default HUD
