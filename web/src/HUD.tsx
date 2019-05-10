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
import { ResourceView } from "./types"
import ErrorPane, { ErrorResource } from "./ErrorPane"
import PreviewList from "./PreviewList"
import { HotKeys } from "react-hotkeys"

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

  keymap() {
    return {
      clearSnackRestarts: "ctrl+shift+9",
    }
  }

  handlers() {
    return {
      clearSnackRestarts: (event: KeyboardEvent | undefined) => {
        if (this.state.View) {
          this.state.View.Resources.forEach(r => {
            fetch(
              this.pathBuilder.path(
                `/api/control/reset_restarts?name=${r.Name}`
              )
            )
          })
        }
      },
    }
  }

  render() {
    let view = this.state.View
    let sailEnabled = view && view.SailEnabled ? view.SailEnabled : false
    let sailUrl = view && view.SailURL ? view.SailURL : ""
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
      let numErrors = 0
      if (name !== "") {
        let selectedResource = resources.find(r => r.Name === name)
        let er = new ErrorResource(selectedResource)
        if (er.hasError()) {
          numErrors = 1
        }
      } else {
        numErrors = resourcesWithErrors.length
      }
      return (
        <TopBar
          logUrl={name === "" ? this.path("/") : this.path(`/r/${name}`)}
          errorsUrl={
            name === "" ? this.path("/errors") : this.path(`/r/${name}/errors`)
          }
          previewUrl={this.path(this.getPreviewForName(name, sidebarItems))}
          resourceView={t}
          sailEnabled={sailEnabled}
          sailUrl={sailUrl}
          numberOfErrors={numErrors}
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
      let podID: string = ""
      if (view && name !== "") {
        let r = view.Resources.find(r => r.Name === name)
        logs = (r && r.CombinedLog) || ""
        endpoints = (r && r.Endpoints) || []
        podID = (r && r.PodID) || ""
      }
      return (
        <LogPane
          log={logs}
          isExpanded={isSidebarClosed}
          endpoints={endpoints}
          podID={podID}
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
        return <ErrorPane resources={[new ErrorResource(er)]} />
      }
      return <ErrorPane resources={[]} />
    }
    let errorResources = resources.map(r => new ErrorResource(r))
    let resourcesWithErrors = errorResources.filter(r => r.hasError())

    return (
      <HotKeys keyMap={this.keymap()} handlers={this.handlers()}>
        <div className="HUD">
          <Switch>
            <Route
              path={this.path("/r/:name/errors")}
              render={topBarRoute.bind(null, ResourceView.Errors)}
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
              path={this.path("/errors")}
              render={topBarRoute.bind(null, ResourceView.Errors)}
            />
            <Route render={topBarRoute.bind(null, ResourceView.Log)} />
          </Switch>
          <Switch>
            <Route
              path={this.path("/r/:name/errors")}
              render={sidebarRoute.bind(null, ResourceView.Errors)}
            />
            <Route
              path={this.path("/errors")}
              render={sidebarRoute.bind(null, ResourceView.Errors)}
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
          <Statusbar items={statusItems} errorsUrl={this.path("/errors")} />
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
                />
              )}
            />
            <Route
              exact
              path={this.path("/errors")}
              render={() => <ErrorPane resources={errorResources} />}
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
              path={this.path("/r/:name/errors")}
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
      </HotKeys>
    )
  }
}

export default HUD
