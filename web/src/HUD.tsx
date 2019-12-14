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
import { matchPath } from "react-router"
import { Route, Switch, RouteComponentProps } from "react-router-dom"
import { History, UnregisterCallback } from "history"
import { incr, pathToTag } from "./analytics"
import TopBar from "./TopBar"
import SocketBar from "./SocketBar"
import "./HUD.scss"
import {
  LogLine,
  ResourceView,
  ShowFatalErrorModal,
  SnapshotHighlight,
  SocketState,
} from "./types"
import { logLinesFromString } from "./logs"
import HudState from "./HudState"
import AlertPane from "./AlertPane"
import AnalyticsNudge from "./AnalyticsNudge"
import NotFound from "./NotFound"
import { numberOfAlerts } from "./alerts"
import Features from "./feature"
import ShareSnapshotModal from "./ShareSnapshotModal"
import FatalErrorModal from "./FatalErrorModal"
import * as _ from "lodash"
import FacetsPane from "./FacetsPane"
import HUDGrid from "./HUDGrid"
import LogStore from "./LogStore"

type HudProps = {
  history: History
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
        versionSettings: { checkUpdates: true },
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
    this.handleOpenModal = this.handleOpenModal.bind(this)
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
    this.setState(prevState => {
      let newState = _.clone(state) as any
      newState.logStore = prevState.logStore ?? new LogStore()

      let newLogList = newState.view?.logList
      if (newLogList) {
        let fromCheckpoint = newLogList.fromCheckpoint ?? 0
        if (fromCheckpoint > 0) {
          newState.logStore.append(newLogList)
        } else if (fromCheckpoint === 0) {
          // if the fromCheckpoint is 0 or undefined, create a brand new log store.
          newState.logStore = new LogStore()
          newState.logStore.append(newLogList)
        }
      }
      return newState
    })
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

  private handleOpenModal() {
    this.setState({ showSnapshotModal: true })
  }

  render() {
    let view = this.state.view

    let needsNudge = view?.needsAnalyticsNudge ?? false
    let logStore = this.state.logStore ?? null
    let resources = view?.resources ?? []
    if (!resources?.length) {
      return <HeroScreen message={"Loading…"} />
    }
    let statusItems = resources.map(res => new StatusItem(res))

    let runningVersion = view?.runningTiltBuild
    let latestVersion = view?.latestTiltBuild
    const versionSettings = view?.versionSettings
    const checkUpdates = versionSettings?.checkUpdates ?? true
    let shareSnapshotModal = this.renderShareSnapshotModal(view)
    let fatalErrorModal = this.renderFatalErrorModal(view)

    let statusbar = (
      <Statusbar
        items={statusItems}
        alertsUrl={this.path("/alerts")}
        runningVersion={runningVersion}
        latestVersion={latestVersion}
        checkVersion={checkUpdates}
      />
    )

    let hudClasses = ["HUD"]
    if (this.pathBuilder.isSnapshot()) {
      hudClasses.push("is-snapshot")
    }

    return (
      <div className={hudClasses.join(" ")}>
        <AnalyticsNudge needsNudge={needsNudge} />
        <SocketBar state={this.state.socketState} />
        {fatalErrorModal}
        {shareSnapshotModal}

        {this.renderSidebarSwitch()}
        {statusbar}

        <HUDGrid
          topBar={this.renderTopBarSwitch()}
          resourceBar={this.renderResourceBar()}
          isSidebarClosed={!!this.state.isSidebarClosed}
        >
          {this.renderMainPaneSwitch()}
        </HUDGrid>
      </div>
    )
  }

  renderResourceBar() {
    let match = matchPath(String(this.props.history.location.pathname), {
      path: this.path("/r/:name"),
      exact: true,
    })
    let params: any = match?.params
    let name = params?.name
    if (!name) {
      return null
    }

    let view = this.state.view
    let resources = view?.resources ?? []
    let r = resources.find(r => r.name === name)
    if (!r) {
      return null
    }

    let endpoints = r?.endpoints ?? []
    let podID = r?.podID ?? ""
    let podStatus = (r.k8sResourceInfo && r.k8sResourceInfo.podStatus) || ""
    return (
      <ResourceInfo endpoints={endpoints} podID={podID} podStatus={podStatus} />
    )
  }

  renderTopBarSwitch() {
    let view = this.state.view
    let resources = view?.resources ?? []
    let showSnapshot =
      this.getFeatures().isEnabled("snapshots") &&
      !this.pathBuilder.isSnapshot()

    let snapshotHighlight = this.state.snapshotHighlight || null

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
              handleOpenModal={this.handleOpenModal}
              highlight={snapshotHighlight}
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
      let isFacetsEnabled = this.getFeatures().isEnabled("facets")
      return (
        <TopBar
          logUrl={name === "" ? this.path("/") : this.path(`/r/${name}`)}
          alertsUrl={
            name === "" ? this.path("/alerts") : this.path(`/r/${name}/alerts`)
          }
          resourceView={t}
          numberOfAlerts={numAlerts}
          showSnapshotButton={showSnapshot}
          handleOpenModal={this.handleOpenModal}
          highlight={snapshotHighlight}
          facetsUrl={
            name !== "" && isFacetsEnabled
              ? this.path(`/r/${name}/facets`)
              : null
          }
        />
      )
    }

    return (
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
    )
  }

  renderSidebarSwitch() {
    let view = this.state.view
    let resources = (view && view.resources) || []
    let sidebarItems = resources.map(res => new SidebarItem(res))
    let isSidebarClosed = !!this.state.isSidebarClosed
    let sidebarRoute = (t: ResourceView, props: RouteComponentProps<any>) => {
      let name = props.match.params.name
      return (
        <Sidebar
          selected={name}
          items={sidebarItems}
          isClosed={isSidebarClosed}
          toggleSidebar={this.toggleSidebar}
          resourceView={t}
          pathBuilder={this.pathBuilder}
        />
      )
    }
    return (
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
    )
  }

  renderMainPaneSwitch() {
    let logStore = this.state.logStore ?? null
    let view = this.state.view
    let resources = (view && view.resources) || []
    let snapshotHighlight = this.state.snapshotHighlight || null
    let showSnapshotModal = !!this.state.showSnapshotModal
    let isSnapshot = this.pathBuilder.isSnapshot()
    let logsRoute = (props: RouteComponentProps<any>) => {
      let name =
        props.match.params && props.match.params.name
          ? props.match.params.name
          : ""

      let r = resources.find(r => r.name === name)
      if (r === undefined) {
        return <Route component={NotFound} />
      }

      let logLines: LogLine[] = []
      if (name && logStore) {
        logLines = logStore.manifestLog(name)
      }

      return (
        <LogPane
          logLines={logLines}
          showManifestPrefix={false}
          handleSetHighlight={this.handleSetHighlight}
          handleClearHighlight={this.handleClearHighlight}
          highlight={snapshotHighlight}
          modalIsOpen={showSnapshotModal}
          isSnapshot={isSnapshot}
        />
      )
    }

    let errorRoute = (props: RouteComponentProps<any>) => {
      let name = props.match.params ? props.match.params.name : ""
      let er = resources.find(r => r.name === name)
      if (er === undefined) {
        return <Route component={NotFound} />
      }
      if (er) {
        return (
          <AlertPane
            pathBuilder={this.pathBuilder}
            resources={[er]}
            logStore={logStore}
          />
        )
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
    let allLogsRoute = () => {
      let allLogs: LogLine[] = []
      if (logStore) {
        allLogs = logStore.allLog()
      } else if (view?.log) {
        allLogs = logLinesFromString(
          "ERROR: Tilt Server and client protocol mismatch. This happens in dev mode if you have a new client talking to an old Tilt binary. Please re-compile Tilt"
        )
      }
      return (
        <LogPane
          logLines={allLogs}
          showManifestPrefix={true}
          handleSetHighlight={this.handleSetHighlight}
          handleClearHighlight={this.handleClearHighlight}
          highlight={this.state.snapshotHighlight}
          modalIsOpen={showSnapshotModal}
          isSnapshot={isSnapshot}
        />
      )
    }

    return (
      <Switch>
        <Route exact path={this.path("/")} render={allLogsRoute} />
        <Route
          exact
          path={this.path("/alerts")}
          render={() => (
            <AlertPane
              pathBuilder={this.pathBuilder}
              resources={resources}
              logStore={logStore}
            />
          )}
        />
        <Route exact path={this.path("/r/:name")} render={logsRoute} />
        <Route
          exact
          path={this.path("/r/:name/k8s")}
          render={() => <K8sViewPane />}
        />
        <Route exact path={this.path("/r/:name/alerts")} render={errorRoute} />
        <Route exact path={this.path("/r/:name/facets")} render={facetsRoute} />
        <Route component={NoMatch} />
      </Switch>
    )
  }

  renderShareSnapshotModal(view: Proto.webviewView | null) {
    let handleClose = () =>
      this.setState({ showSnapshotModal: false, snapshotLink: "" })
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

  renderFatalErrorModal(view: Proto.webviewView | null) {
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
