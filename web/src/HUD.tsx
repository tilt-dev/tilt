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
import ErrorPane, { ErrorResource } from "./ErrorPane"

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
    this.history = createBrowserHistory()
    this.unlisten = () => {}

    this.state = {
      Message: "",
      View: {
        Resources: [],
        Log: "",
        LogTimestamps: false,
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
    return this.pathBuilder.path(relPath)
  }

  render() {
    let view = this.state.View
    let sailUrl = (view && view.SailURL) ? view.SailURL : ""
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

    let tabNavRoute = (t: ResourceView, sailUrl: string, props: RouteComponentProps<any>) => {
      let name =
        props.match.params && props.match.params.name
          ? props.match.params.name
          : ""
      return (
        <TabNav
          logUrl={name === "" ? "/" : `/r/${name}`}
          errorsUrl={name === "" ? "/errors" : `/r/${name}/errors`}
          previewUrl={this.getEndpointForName(name, sidebarItems)}
          resourceView={t}
          sailUrl={sailUrl}
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

    let errorRoute = (props: RouteComponentProps<any>) => {
      let name = props.match.params ? props.match.params.name : ""
      let er = resources.find(r => r.Name === name)
      if (er) {
        return <ErrorPane resources={[new ErrorResource(er)]} />
      }
      return <ErrorPane resources={[]} />
    }

    return (
      <Router history={this.history}>
        <div className="HUD">
          <Switch>
            <Route
              path={this.path("/r/:name/errors")}
              render={tabNavRoute.bind(null, ResourceView.Errors, sailUrl)}
            />
            <Route
              path={this.path("/r/:name/preview")}
              render={tabNavRoute.bind(null, ResourceView.Preview, sailUrl)}
            />
            <Route
              path={this.path("/r/:name")}
              render={tabNavRoute.bind(null, ResourceView.Log, sailUrl)}
            />
            <Route
              path={this.path("/errors")}
              render={tabNavRoute.bind(null, ResourceView.Errors, sailUrl)}
            />
            <Route render={tabNavRoute.bind(null, ResourceView.Log, sailUrl)} />
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
            }
            <Route
              path={this.path("/r/:name")}
              render={sidebarRoute.bind(null, ResourceView.Log)}
            />
            <Route render={sidebarRoute.bind(null, ResourceView.Log)} />
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
            <Route
              exact
              path={this.path("/errors")}
              render={() => (
                <ErrorPane
                  resources={resources.map(r => new ErrorResource(r))}
                />
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
              path={this.path("/r/:name/errors")}
              render={errorRoute}
            />
            } />
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
