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
import {
  Router,
  Route,
  Switch,
  RouteComponentProps,
  Link,
} from "react-router-dom"
import { createBrowserHistory, History, UnregisterCallback } from "history"
import { incr, pathToTag } from "./analytics"
import TabNav from "./TabNav"
import "./HUD.scss"
import { ResourceView } from "./types"

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
  // The root of the HUD view, without the slash.
  //
  // TODO(nick): Update all the Link elements to draw from some LinkProvider
  // that understands rootPath.
  private rootPath: string
  private controller: AppController
  private history: History
  private unlisten: UnregisterCallback

  constructor(props: HudProps) {
    super(props)

    let rootPath = ""
    let url = `ws://${window.location.host}/ws/view`
    let roomPath = new RegExp("^/view/(.+)$")
    let roomMatch = roomPath.exec(window.location.pathname)
    if (roomMatch) {
      let roomId = roomMatch[1]
      url = `ws://${window.location.host}/join/${roomId}`
      rootPath = `/view/${roomId}`
    }

    this.controller = new AppController(url, this)
    this.rootPath = rootPath

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

  getEndpointForName(name: string, resources: Array<SidebarItem>): string {
    let endpoint = ""

    if (name) {
      endpoint = `/r/${name}/preview`
    } else if (resources.length) {
      // Pick the first item with an endpoint, or default to the first item
      endpoint = `/r/${resources[0].name}/preview`
      for (let r of resources) {
        if (r.hasEndpoints) {
          endpoint = `/r/${r.name}/preview`
          break
        }
      }
    }
    return endpoint
  }

  path(relPath: string) {
    if (relPath[0] != "/") {
      throw new Error('relPath should start with "/", actual:' + relPath)
    }
    return this.rootPath + relPath
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

    let tabNavRoute = (props: RouteComponentProps<any>) => {
      let name =
        props.match.params && props.match.params.name
          ? props.match.params.name
          : ""
      return (
        <TabNav
          logUrl={name === "" ? "/" : `/r/${name}`}
          previewUrl={this.getEndpointForName(name, sidebarItems)}
          resourceView={ResourceView.Log}
        />
      )
    }
    let tabNavPreviewRoute = (props: RouteComponentProps<any>) => {
      let name =
        props.match.params && props.match.params.name
          ? props.match.params.name
          : ""
      return (
        <TabNav
          logUrl={name === "" ? "/" : `/r/${name}`}
          previewUrl={this.getEndpointForName(name, sidebarItems)}
          resourceView={ResourceView.Preview}
        />
      )
    }

    let logsRoute = (props: RouteComponentProps<any>) => {
      let name =
        props.match.params && props.match.params.name
          ? props.match.params.name
          : ""
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
            <Route
              path={this.path("/r/:name/preview")}
              render={tabNavPreviewRoute}
            />
            <Route path={this.path("/r/:name")} render={tabNavRoute} />
            <Route render={tabNavRoute} />
          </Switch>
          <Switch>
            <Route
              path={this.path("/r/:name/preview")}
              render={sidebarPreviewRoute}
            />
            }
            <Route path={this.path("/r/:name")} render={sidebarRoute} />
            <Route render={sidebarRoute} />
          </Switch>
          <Statusbar items={statusItems} />
          <Switch>
            <Route
              exact
              path={this.path("/")}
              render={() => (
                <LogPane log={combinedLog} isExpanded={isSidebarClosed} />
              )}
            />
            <Route exact path={this.path("/r/:name")} render={logsRoute} />
            <Route
              exact
              path={this.path("/r/:name/k8s")}
              render={() => <K8sViewPane />}
            />
            <Route
              exact
              path={this.path("/r/:name/preview")}
              render={previewRoute}
            />
            <Route component={NoMatch} />
          </Switch>
        </div>
      </Router>
    )
  }
}

export default HUD
