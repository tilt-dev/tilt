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
  BrowserRouter as Router,
  Route,
  Switch,
  RouteComponentProps,
} from "react-router-dom"
import "./HUD.scss"

interface HudProps extends RouteComponentProps<any> {}

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
    TiltfileErrorMessage: string
  } | null
  isSidebarOpen: boolean
}

// The Main HUD view, as specified in
// https://docs.google.com/document/d/1VNIGfpC4fMfkscboW0bjYYFJl07um_1tsFrbN-Fu3FI/edit#heading=h.l8mmnclsuxl1
class HUD extends Component<HudProps, HudState> {
  private controller: AppController

  constructor(props: HudProps) {
    super(props)

    this.controller = new AppController(
      `ws://${window.location.host}/ws/view`,
      this
    )
    this.state = {
      Message: "",
      View: {
        Resources: [],
        Log: "",
        LogTimestamps: false,
        TiltfileErrorMessage: "",
      },
      isSidebarOpen: false,
    }

    this.toggleSidebar = this.toggleSidebar.bind(this)
  }

  componentDidMount() {
    this.controller.createNewSocket()
  }

  componentWillUnmount() {
    this.controller.dispose()
  }

  setAppState(state: HudState) {
    this.setState(state)
  }

  toggleSidebar() {
    this.setState(prevState => {
      return Map(prevState)
        .set("isSidebarOpen", !prevState.isSidebarOpen)
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

    let isSidebarOpen = this.state.isSidebarOpen
    let statusItems = resources.map(res => new StatusItem(res))
    let sidebarItems = resources.map(res => new SidebarItem(res))
    let SidebarRoute = function(props: HudProps) {
      let name = props.match.params.name
      return (
        <Sidebar selected={name} items={sidebarItems} isOpen={isSidebarOpen} />
      )
    }

    let LogsRoute = (props: HudProps) => {
      let name = props.match.params ? props.match.params.name : ""
      let logs = ""
      if (view && name !== "") {
        let r = view.Resources.find(r => r.Name === name)
        logs = r ? r.CombinedLog : ""
      }
      return <LogPane log={logs} />
    }

    let combinedLog = ""
    if (view) {
      combinedLog = view.Log
    }

    return (
      <Router>
        <div className="HUD">
          <Switch>
            <Route path="/r/:name" component={SidebarRoute} />
            <Route component={SidebarRoute} />
          </Switch>

          <Statusbar items={statusItems} toggleSidebar={this.toggleSidebar} />
          <Switch>
            <Route
              exact
              path="/"
              render={() => <LogPane log={combinedLog} />}
            />
            <Route exact path="/r/:name" component={LogsRoute} />
            <Route exact path="/r/:name/k8s" render={() => <K8sViewPane />} />
            <Route
              exact
              path="/r/:name/preview"
              render={() => <PreviewPane />}
            />
            <Route component={NoMatch} />
          </Switch>
        </div>
      </Router>
    )
  }
}

export default HUD
