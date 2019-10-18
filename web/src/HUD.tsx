import React, { Component } from "react"
import AppController from "./AppController"
import NoMatch from "./NoMatch"
import LoadingScreen from "./LoadingScreen"
import Sidebar, { SidebarItem } from "./Sidebar"
import Statusbar, { StatusItem } from "./Statusbar"
import LogPane from "./LogPane"
import ResourceInfo from "./ResourceInfo"
import K8sViewPane from "./K8sViewPane"
import PathBuilder from "./PathBuilder"
import { Route, Switch, RouteComponentProps } from "react-router-dom"
import { History, UnregisterCallback } from "history"
import { incr, pathToTag } from "./analytics"
import TopBar from "./TopBar"
import "./HUD.scss"
import {
  TiltBuild,
  ResourceView,
  Resource,
  Snapshot,
  ShowFatalErrorModal,
  SnapshotHighlight,
} from "./types"
import AlertPane from "./AlertPane"
import AnalyticsNudge from "./AnalyticsNudge"
import NotFound from "./NotFound"
import { numberOfAlerts } from "./alerts"
import Features from "./feature"
import ShareSnapshotModal from "./ShareSnapshotModal"
import cleanStateForSnapshotPOST from "./snapshot_sanitizer"
import FatalErrorModal from "./FatalErrorModal"

type HudProps = {
  history: History
}

type HudView = {
  Resources: Array<Resource>
  Log: string
  LogTimestamps: boolean
  NeedsAnalyticsNudge: boolean
  RunningTiltBuild: TiltBuild
  LatestTiltBuild: TiltBuild
  FeatureFlags: { [featureFlag: string]: boolean }
  TiltCloudUsername: string
  TiltCloudSchemeHost: string
  TiltCloudTeamID: string
  FatalError: string | null
}

type HudState = {
  Message: string
  View: HudView | null
  IsSidebarClosed: boolean
  SnapshotLink: string
  showSnapshotModal: boolean
  showFatalErrorModal: ShowFatalErrorModal
  snapshotHighlight: SnapshotHighlight | null
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
    this.controller = new AppController(this.pathBuilder, this)
    this.history = props.history
    this.unlisten = this.history.listen((location, _) => {
      let tags = { type: pathToTag(location.pathname) }
      incr("ui.web.navigation", tags)

      this.handleClearHighlight()
      let selection = document.getSelection()
      selection && selection.removeAllRanges()
    })

    this.state = {
      Message: "",
      View: {
        Resources: [],
        Log: "",
        LogTimestamps: false,
        NeedsAnalyticsNudge: false,
        FatalError: null,
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
        TiltCloudTeamID: "",
      },
      IsSidebarClosed: false,
      SnapshotLink: "",
      showSnapshotModal: false,
      showFatalErrorModal: ShowFatalErrorModal.Default,
      snapshotHighlight: null,
    }

    this.toggleSidebar = this.toggleSidebar.bind(this)
    this.handleClearHighlight = this.handleClearHighlight.bind(this)
    this.handleSetHighlight = this.handleSetHighlight.bind(this)
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

  setHistoryLocation(path: string) {
    this.props.history.replace(path)
  }

  toggleSidebar() {
    this.setState(prevState => {
      return {
        IsSidebarClosed: !prevState.IsSidebarClosed,
      }
    })
  }

  path(relPath: string) {
    return this.pathBuilder.path(relPath)
  }

  sendSnapshot(snapshot: Snapshot) {
    let url = `//${window.location.host}/api/snapshot/new`
    let sanitizedSnapshot = cleanStateForSnapshotPOST(snapshot)
    sanitizedSnapshot.path = this.props.history.location.pathname
    sanitizedSnapshot.snapshotHighlight = this.state.snapshotHighlight
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
      .catch(err => console.error(err))
  }

  private getFeatures(): Features {
    if (this.state.View) {
      return new Features(this.state.View.FeatureFlags)
    }

    return new Features({})
  }

  handleSetHighlight(highlight: SnapshotHighlight) {
    this.setState({
      snapshotHighlight: highlight,
    })
  }

  handleClearHighlight() {
    this.setState({
      snapshotHighlight: null,
    })
  }

  render() {
    let view = this.state.View
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

    let showSnapshot =
      this.getFeatures().isEnabled("snapshots") &&
      !this.pathBuilder.isSnapshot()
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

    let handleOpenModal = () => {
      this.setState({ showSnapshotModal: true })
    }
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
              resourceView={t}
              numberOfAlerts={numAlerts}
              showSnapshotButton={showSnapshot}
              snapshotOwner={snapshotOwner}
              handleOpenModal={handleOpenModal}
              highlight={this.state.snapshotHighlight}
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
          resourceView={t}
          numberOfAlerts={numAlerts}
          showSnapshotButton={showSnapshot}
          snapshotOwner={snapshotOwner}
          handleOpenModal={handleOpenModal}
          highlight={this.state.snapshotHighlight}
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
        podStatus = (r.K8sResourceInfo && r.K8sResourceInfo.PodStatus) || ""
      }
      return (
        <>
          <ResourceInfo
            endpoints={endpoints}
            podID={podID}
            podStatus={podStatus}
          />
          <LogPane
            log={logs}
            isExpanded={isSidebarClosed}
            handleSetHighlight={this.handleSetHighlight}
            handleClearHighlight={this.handleClearHighlight}
            highlight={this.state.snapshotHighlight}
            modalIsOpen={this.state.showSnapshotModal}
          />
        </>
      )
    }

    let combinedLog = ""
    if (view) {
      combinedLog = view.Log
    }

    let errorRoute = (props: RouteComponentProps<any>) => {
      let name = props.match.params ? props.match.params.name : ""
      let er = resources.find(r => r.Name === name)
      if (er === undefined) {
        return <Route component={NotFound} />
      }
      if (er) {
        return <AlertPane pathBuilder={this.pathBuilder} resources={[er]} />
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
    let fatalErrorModal = this.renderFatalErrorModal(view)
    return (
      <div className="HUD">
        <AnalyticsNudge needsNudge={needsNudge} />
        {fatalErrorModal}
        {shareSnapshotModal}
        <Switch>
          <Route
            path={this.path("/r/:name/alerts")}
            render={topBarRoute.bind(null, ResourceView.Alerts)}
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
                handleSetHighlight={this.handleSetHighlight}
                handleClearHighlight={this.handleClearHighlight}
                highlight={this.state.snapshotHighlight}
                modalIsOpen={this.state.showSnapshotModal}
              />
            )}
          />
          <Route
            exact
            path={this.path("/alerts")}
            render={() => (
              <AlertPane pathBuilder={this.pathBuilder} resources={resources} />
            )}
          />
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
    let tiltCloudTeamID = (view && view.TiltCloudTeamID) || null
    let highlightedLines = this.state.snapshotHighlight
      ? Math.abs(
          parseInt(this.state.snapshotHighlight.endingLogID, 10) -
            parseInt(this.state.snapshotHighlight.beginningLogID, 10)
        ) + 1
      : null
    return (
      <ShareSnapshotModal
        handleSendSnapshot={handleSendSnapshot}
        handleClose={handleClose}
        snapshotUrl={this.state.SnapshotLink}
        tiltCloudUsername={tiltCloudUsername}
        tiltCloudSchemeHost={tiltCloudSchemeHost}
        tiltCloudTeamID={tiltCloudTeamID}
        isOpen={this.state.showSnapshotModal}
        highlightedLines={highlightedLines}
      />
    )
  }

  renderFatalErrorModal(view: HudView | null) {
    let error = view && view.FatalError
    let handleClose = () =>
      this.setState({ showFatalErrorModal: ShowFatalErrorModal.Hide })
    return (
      <FatalErrorModal
        error={error}
        showFatalErrorModal={this.state.showFatalErrorModal}
        handleClose={handleClose}
      />
    )
  }
}

export default HUD
