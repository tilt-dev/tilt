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
import { Route, Switch, RouteComponentProps } from "react-router-dom"
import { History, UnregisterCallback } from "history"
import { incr, pathToTag } from "./analytics"
import TopBar from "./TopBar"
import "./HUD.scss"
import { TiltBuild, ResourceView, Resource, Snapshot } from "./types"
import AlertPane from "./AlertPane"
import PreviewList from "./PreviewList"
import AnalyticsNudge from "./AnalyticsNudge"
import NotFound from "./NotFound"
import { numberOfAlerts, isK8sResourceInfo } from "./alerts"
import Features from "./feature"
import ShareSnapshotModal from "./ShareSnapshotModal"
import cleanStateForSnapshotPOST from "./snapshot_sanitizer"

type HudProps = {
  history: History
}

type HudView = {
  Resources: Array<Resource>
  Log: string
  LogTimestamps: boolean
  SailEnabled: boolean
  SailURL: string
  NeedsAnalyticsNudge: boolean
  RunningTiltBuild: TiltBuild
  LatestTiltBuild: TiltBuild
  FeatureFlags: { [featureFlag: string]: boolean }
  TiltCloudUsername: string
  TiltCloudSchemeHost: string
}

type HudState = {
  Message: string
  View: HudView | null
  IsSidebarClosed: boolean
  SnapshotLink: string
  showSnapshotModal: boolean
}

type NewSnapshotResponse = {
  // output of snapshot_storage
  url: string
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

    incr("ui.web.init", { ua: window.navigator.userAgent })

    this.pathBuilder = new PathBuilder(
      window.location.host,
      window.location.pathname
    )
    this.controller = new AppController(this.pathBuilder.getDataUrl(), this)
    this.history = props.history
    this.unlisten = this.history.listen((location, _) => {
      let tags = { type: pathToTag(location.pathname) }
      incr("ui.web.navigation", tags)
    })

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
        FeatureFlags: {},
        TiltCloudUsername: "",
        TiltCloudSchemeHost: "",
      },
      IsSidebarClosed: false,
      SnapshotLink: "",
      showSnapshotModal: false,
    }

    this.toggleSidebar = this.toggleSidebar.bind(this)
  }

  componentDidMount() {
    if (process.env.NODE_ENV === "test") {
      // we don't want to run any bootstrapping code in the test environment
      return
    }
    if (this.pathBuilder.isSnapshot()) {
      this.controller.setStateFromSnapshot()
    } else {
      this.controller.createNewSocket()
    }
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
      return {
        IsSidebarClosed: !prevState.IsSidebarClosed,
      }
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

  sendSnapshot(snapshot: Snapshot) {
    let url = `//${window.location.host}/api/snapshot/new`
    let sanitizedSnapshot = cleanStateForSnapshotPOST(snapshot)
    fetch(url, {
      method: "post",
      body: JSON.stringify(sanitizedSnapshot),
    })
      .then(res => {
        res
          .json()
          .then((value: NewSnapshotResponse) => {
            this.setState({
              SnapshotLink: value.url,
            })
          })
          .catch(err => console.error(err))
      })
      .then(err => console.error(err))
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
    var features: Features
    if (this.state.View) {
      features = new Features(this.state.View.FeatureFlags)
    } else {
      features = new Features({})
    }
    let showSnapshot =
      features.isEnabled("snapshots") && !this.pathBuilder.isSnapshot()
    let snapshotOwner: string | null = null
    if (this.pathBuilder.isSnapshot() && this.state.View) {
      snapshotOwner = this.state.View.TiltCloudUsername
    }

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

    let handleOpenModal = () => this.setState({ showSnapshotModal: true })

    let topBarRoute = (t: ResourceView, props: RouteComponentProps<any>) => {
      let name =
        props.match.params && props.match.params.name
          ? props.match.params.name
          : ""
      let numAlerts = 0
      if (name) {
        let selectedResource = resources.find(r => r.Name === name)
        if (selectedResource === undefined) {
          return (
            <TopBar
              logUrl={this.path("/")} // redirect to home page
              alertsUrl={this.path("/alerts")}
              previewUrl={this.path("/preview")}
              resourceView={t}
              sailEnabled={sailEnabled}
              sailUrl={sailUrl}
              numberOfAlerts={numAlerts}
              showSnapshotButton={showSnapshot}
              snapshotOwner={snapshotOwner}
              handleOpenModal={handleOpenModal}
            />
          )
        }
        numAlerts = numberOfAlerts(selectedResource)
      } else {
        numAlerts = resources
          .map(r => numberOfAlerts(r))
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
          showSnapshotButton={showSnapshot}
          snapshotOwner={snapshotOwner}
          handleOpenModal={handleOpenModal}
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
      if (view && name) {
        let r = view.Resources.find(r => r.Name === name)
        if (r === undefined) {
          return <Route component={NotFound} />
        }
        logs = (r && r.CombinedLog) || ""
        endpoints = (r && r.Endpoints) || []
        podID = (r && r.PodID) || ""
        if (isK8sResourceInfo(r.ResourceInfo)) {
          podStatus = (r && r.ResourceInfo && r.ResourceInfo.PodStatus) || ""
        }
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
      if (view && name) {
        let r = view.Resources.find(r => r.Name === name)
        if (r === undefined) {
          return <Route component={NotFound} />
        }
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
      if (er === undefined) {
        return <Route component={NotFound} />
      }
      if (er) {
        return <AlertPane resources={[er]} />
      }
    }
    let snapshotRoute = () => {
      return (
        <div className="SnapshotMessage">
          <h1>Welcome to a Tilt snapshot!</h1>
          <p>
            In here you can look around and check out a "snapshot" of a Tilt
            session.
          </p>
          <p>
            Snapshots are static freeze frame points in time. Nothing will
            change. Feel free to poke around and see what the person who sent
            you this snapshot saw when they sent it to you.
          </p>
          <p>Have fun!</p>
        </div>
      )
    }
    let runningVersion = view && view.RunningTiltBuild
    let latestVersion = view && view.LatestTiltBuild
    let shareSnapshotModal = this.renderShareSnapshotModal(view)
    return (
      <div className="HUD">
        <AnalyticsNudge needsNudge={needsNudge} />
        {shareSnapshotModal}
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
            render={() => <AlertPane resources={resources} />}
          />
          <Route exact path={this.path("/preview")} render={previewRoute} />
          <Route exact path={this.path("/r/:name")} render={logsRoute} />
          <Route
            exact
            path={this.path("/snapshot/:snap_id")}
            render={snapshotRoute}
          />
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

  renderShareSnapshotModal(view: HudView | null) {
    let handleClose = () => this.setState({ showSnapshotModal: false })
    let handleSendSnapshot = () => this.sendSnapshot(this.state)
    let tiltCloudUsername = (view && view.TiltCloudUsername) || null
    let tiltCloudSchemeHost = (view && view.TiltCloudSchemeHost) || ""
    return (
      <ShareSnapshotModal
        handleSendSnapshot={handleSendSnapshot}
        handleClose={handleClose}
        snapshotUrl={this.state.SnapshotLink}
        tiltCloudUsername={tiltCloudUsername}
        tiltCloudSchemeHost={tiltCloudSchemeHost}
        isOpen={this.state.showSnapshotModal}
      />
    )
  }
}

export default HUD
