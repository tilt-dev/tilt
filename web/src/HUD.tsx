import React, { Component } from "react"
import AppController from "./AppController"
import NoMatch from "./NoMatch"
import Sidebar, { SidebarItem } from "./Sidebar"
import Statusbar, { StatusItem } from "./Statusbar"
import LogPane from "./LogPane"
import HeroScreen from "./HeroScreen"
import ResourceInfo from "./ResourceInfo"
import K8sViewPane from "./K8sViewPane"
import PathBuilder from "./PathBuilder"
import { Route, Switch, RouteComponentProps } from "react-router-dom"
import { History, UnregisterCallback } from "history"
import { incr, pathToTag } from "./analytics"
import TopBar from "./TopBar"
import SocketBar from "./SocketBar"
import "./HUD.scss"
import {
  HudState,
  ResourceView,
  ShowFatalErrorModal,
  SnapshotHighlight,
  SocketState,
  WebView,
} from "./types"
import AlertPane from "./AlertPane"
import AnalyticsNudge from "./AnalyticsNudge"
import NotFound from "./NotFound"
import { numberOfAlerts } from "./alerts"
import Features from "./feature"
import ShareSnapshotModal from "./ShareSnapshotModal"
import FatalErrorModal from "./FatalErrorModal"
import * as _ from "lodash"
import FacetsPane from "./FacetsPane"

type HudProps = {
  history: History
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
      view: {
        resources: [],
        log: "",
        logTimestamps: false,
        needsAnalyticsNudge: false,
        fatalError: undefined,
        runningTiltBuild: {
          version: "",
          date: "",
          dev: false,
        },
        latestTiltBuild: {
          version: "",
          date: "",
          dev: false,
        },
        featureFlags: {},
        tiltCloudUsername: "",
        tiltCloudSchemeHost: "",
        tiltCloudTeamID: "",
      },
      isSidebarClosed: false,
      snapshotLink: "",
      showSnapshotModal: false,
      showFatalErrorModal: ShowFatalErrorModal.Default,
      snapshotHighlight: undefined,
      socketState: SocketState.Closed,
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

  setAppState<K extends keyof HudState>(state: Pick<HudState, K>) {
    this.setState(state)
  }

  setHistoryLocation(path: string) {
    this.props.history.replace(path)
  }

  toggleSidebar() {
    this.setState(prevState => {
      return {
        isSidebarClosed: !prevState.isSidebarClosed,
      }
    })
  }

  path(relPath: string) {
    return this.pathBuilder.path(relPath)
  }

  snapshotFromState(state: HudState): Proto.webviewSnapshot {
    return {
      view: _.cloneDeep(state.view || null),
      isSidebarClosed: !!state.isSidebarClosed,
      path: this.props.history.location.pathname,
      snapshotHighlight: _.cloneDeep(state.snapshotHighlight),
    }
  }

  sendSnapshot(snapshot: Proto.webviewSnapshot) {
    let url = `//${window.location.host}/api/snapshot/new`

    if (!snapshot.view) {
      return
    }

    let body = JSON.stringify(snapshot)

    fetch(url, {
      method: "post",
      body: body,
    })
      .then(res => {
        res
          .json()
          .then((value: Proto.webviewUploadSnapshotResponse) => {
            this.setState({
              snapshotLink: value.url ? value.url : "",
            })
          })
          .catch(err => console.error(err))
      })
      .catch(err => console.error(err))
  }

  private getFeatures(): Features {
    if (this.state.view) {
      return new Features(this.state.view.featureFlags)
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
      snapshotHighlight: undefined,
    })
  }

  getTeamUpdatesUrl(): string {
    let isUpdateHistoryEnabled = this.getFeatures().isEnabled("update_history")
    if (!isUpdateHistoryEnabled) {
      return ""
    }
    let view = this.state.view
    let teamId = view.tiltCloudTeamID
    if (!teamId) {
      return ""
    }
    let tiltCloudSchemeHost = view.tiltCloudSchemeHost || ""
    return `${tiltCloudSchemeHost}/team/${teamId}/updates`
  }

  getTeamSnapshotsUrl(): string {
    let view = this.state.view
    let teamId = view.tiltCloudTeamID
    if (!teamId) {
      return ""
    }
    let tiltCloudSchemeHost = view.tiltCloudSchemeHost || ""
    return `${tiltCloudSchemeHost}/team/${teamId}/snapshots`
  }

  render() {
    let view = this.state.view

    let needsNudge = view ? view.needsAnalyticsNudge : false
    let resources = (view && view.resources) || []
    if (!resources.length) {
      return <HeroScreen message={"Loading…"} />
    }
    let isSidebarClosed = !!this.state.isSidebarClosed
    let snapshotHighlight = this.state.snapshotHighlight || null
    let showSnapshotModal = !!this.state.showSnapshotModal
    let toggleSidebar = this.toggleSidebar
    let statusItems = resources.map(res => new StatusItem(res))
    let sidebarItems = resources.map(res => new SidebarItem(res))

    let showSnapshot =
      this.getFeatures().isEnabled("snapshots") &&
      !this.pathBuilder.isSnapshot()
    let snapshotOwner: string | null = null
    if (this.pathBuilder.isSnapshot() && this.state.view) {
      snapshotOwner = this.state.view.tiltCloudUsername
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
        let selectedResource = resources.find(r => r.name === name)
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
              highlight={snapshotHighlight}
              teamSnapshotsUrl={this.getTeamSnapshotsUrl()}
              teamUpdatesUrl={this.getTeamUpdatesUrl()}
              facetsUrl={null}
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
          highlight={snapshotHighlight}
          teamSnapshotsUrl={this.getTeamSnapshotsUrl()}
          teamUpdatesUrl={this.getTeamUpdatesUrl()}
          facetsUrl={
            name !== "" &&
            this.state.view.featureFlags &&
            this.state.view.featureFlags["facets"]
              ? this.path(`/r/${name}/facets`)
              : null
          }
        />
      )
    }

    let isSnapshot = this.pathBuilder.isSnapshot()

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
        let r = view.resources.find(r => r.name === name)
        if (r === undefined) {
          return <Route component={NotFound} />
        }
        logs = (r && r.combinedLog) || ""
        endpoints = (r && r.endpoints) || []
        podID = (r && r.podID) || ""
        podStatus = (r.k8sResourceInfo && r.k8sResourceInfo.podStatus) || ""
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
            highlight={snapshotHighlight}
            modalIsOpen={showSnapshotModal}
            isSnapshot={isSnapshot}
          />
        </>
      )
    }

    let combinedLog = ""
    if (view) {
      combinedLog = view.log
    }

    let errorRoute = (props: RouteComponentProps<any>) => {
      let name = props.match.params ? props.match.params.name : ""
      let er = resources.find(r => r.name === name)
      if (er === undefined) {
        return <Route component={NotFound} />
      }
      if (er) {
        return <AlertPane pathBuilder={this.pathBuilder} resources={[er]} />
      }
    }
    let facetsRoute = (props: RouteComponentProps<any>) => {
      let name = props.match.params ? props.match.params.name : ""
      let fr = resources.find(r => r.name === name)
      if (fr === undefined) {
        return <Route component={NotFound} />
      }
      if (fr) {
        return <FacetsPane resource={fr} />
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
    let runningVersion = view && view.runningTiltBuild
    let latestVersion = view && view.latestTiltBuild
    let shareSnapshotModal = this.renderShareSnapshotModal(view)
    let fatalErrorModal = this.renderFatalErrorModal(view)
    return (
      <div className="HUD">
        <AnalyticsNudge needsNudge={needsNudge} />
        <SocketBar state={this.state.socketState} />
        {fatalErrorModal}
        {shareSnapshotModal}
        <Switch>
          <Route
            path={this.path("/r/:name/alerts")}
            render={topBarRoute.bind(null, ResourceView.Alerts)}
          />
          <Route
            path={this.path("/r/:name/facets")}
            render={topBarRoute.bind(null, ResourceView.Facets)}
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
            path={this.path("/r/:name/facets")}
            render={sidebarRoute.bind(null, ResourceView.Facets)}
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
                modalIsOpen={showSnapshotModal}
                isSnapshot={isSnapshot}
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
          <Route
            exact
            path={this.path("/r/:name/facets")}
            render={facetsRoute}
          />
          <Route component={NoMatch} />
        </Switch>
      </div>
    )
  }

  renderShareSnapshotModal(view: WebView | null) {
    let handleClose = () => this.setState({ showSnapshotModal: false })
    let handleSendSnapshot = () =>
      this.sendSnapshot(this.snapshotFromState(this.state))
    let tiltCloudUsername = (view && view.tiltCloudUsername) || null
    let tiltCloudSchemeHost = (view && view.tiltCloudSchemeHost) || ""
    let tiltCloudTeamID = (view && view.tiltCloudTeamID) || null
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
        snapshotUrl={this.state.snapshotLink}
        tiltCloudUsername={tiltCloudUsername}
        tiltCloudSchemeHost={tiltCloudSchemeHost}
        tiltCloudTeamID={tiltCloudTeamID}
        isOpen={this.state.showSnapshotModal}
        highlightedLines={highlightedLines}
      />
    )
  }

  renderFatalErrorModal(view: WebView | null) {
    let error = view && view.fatalError
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
