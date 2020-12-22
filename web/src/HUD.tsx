import { History, UnregisterCallback } from "history"
import * as _ from "lodash"
import React, { Component } from "react"
import { matchPath } from "react-router"
import { Route, RouteComponentProps, Switch } from "react-router-dom"
import AlertPane from "./AlertPane"
import { combinedAlerts } from "./alerts"
import { incr, pathToTag } from "./analytics"
import AnalyticsNudge from "./AnalyticsNudge"
import AppController from "./AppController"
import ErrorModal from "./ErrorModal"
import FacetsPane from "./FacetsPane"
import FatalErrorModal from "./FatalErrorModal"
import Features from "./feature"
import HeroScreen from "./HeroScreen"
import "./HUD.scss"
import HUDLayout from "./HUDLayout"
import HudState from "./HudState"
import K8sViewPane from "./K8sViewPane"
import { LocalStorageContextProvider } from "./LocalStorage"
import LogPane from "./LogPane"
import { logLinesFromString } from "./logs"
import LogStore, { LogStoreProvider } from "./LogStore"
import MetricsPane from "./MetricsPane"
import NoMatch from "./NoMatch"
import NotFound from "./NotFound"
import OverviewPane from "./OverviewPane"
import OverviewResourcePane from "./OverviewResourcePane"
import PathBuilder, { PathBuilderProvider } from "./PathBuilder"
import ResourceInfo from "./ResourceInfo"
import SecondaryNav from "./SecondaryNav"
import ShareSnapshotModal from "./ShareSnapshotModal"
import Sidebar from "./Sidebar"
import SidebarAccount from "./SidebarAccount"
import SidebarItem from "./SidebarItem"
import SidebarResources from "./SidebarResources"
import SocketBar from "./SocketBar"
import Statusbar, { StatusItem } from "./Statusbar"
import { traceNav } from "./trace"
import {
  LogLine,
  ResourceView,
  ShowErrorModal,
  ShowFatalErrorModal,
  SnapshotHighlight,
  SocketState,
  TargetType,
} from "./types"

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

    this.pathBuilder = new PathBuilder(window.location)
    this.controller = new AppController(this.pathBuilder, this)
    this.history = props.history
    this.unlisten = this.history.listen((location: any, action: string) => {
      let tags: any = { type: pathToTag(location.pathname) }
      if (action === "PUSH" && location?.state?.action) {
        tags.action = location.state.action
      }
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
        suggestedTiltVersion: "",
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
      showCopySuccess: false,
      snapshotHighlight: undefined,
      socketState: SocketState.Closed,
      showErrorModal: ShowErrorModal.Default,
      error: undefined,
      logStore: new LogStore(),
    }

    this.toggleSidebar = this.toggleSidebar.bind(this)
    this.handleClearHighlight = this.handleClearHighlight.bind(this)
    this.handleSetHighlight = this.handleSetHighlight.bind(this)
    this.handleOpenModal = this.handleOpenModal.bind(this)
    this.handleShowCopySuccess = this.handleShowCopySuccess.bind(this)
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
    this.setState((prevState) => {
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
    this.setState((prevState) => {
      return {
        isSidebarClosed: !prevState.isSidebarClosed,
      }
    })
  }

  path(relPath: string) {
    return this.pathBuilder.path(relPath)
  }

  snapshotFromState(state: HudState): Proto.webviewSnapshot {
    let view = _.cloneDeep(state.view ?? null)
    if (view && state.logStore) {
      view.logList = state.logStore.toLogList()
    }
    return {
      view: view,
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

    // TODO(dmiller): we need to figure out a way to get human readable error messages from the server
    fetch(url, {
      method: "post",
      body: body,
    })
      .then((res) => {
        res
          .json()
          .then((value: Proto.webviewUploadSnapshotResponse) => {
            this.setState({
              snapshotLink: value.url ? value.url : "",
            })
          })
          .catch((err) => {
            console.error(err)
            this.setAppState({
              showSnapshotModal: false,
              error: "Error decoding JSON response",
            })
          })
      })
      .catch((err) => {
        console.error(err)
        this.setAppState({
          showSnapshotModal: false,
          error: "Error posting snapshot",
        })
      })
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

  handleShowCopySuccess() {
    this.setState(
      {
        showCopySuccess: true,
      },
      () => {
        setTimeout(() => {
          this.setState({
            showCopySuccess: false,
          })
        }, 1500)
      }
    )
  }

  private handleOpenModal() {
    this.setState({ showSnapshotModal: true })
  }

  render() {
    let view = this.state.view

    let needsNudge = view?.needsAnalyticsNudge ?? false
    let resources = view?.resources ?? []
    if (!resources?.length || !view?.tiltfileKey) {
      return <HeroScreen message={"Loading…"} />
    }
    let statusItems = resources.map((res) => new StatusItem(res))

    let runningBuild = view?.runningTiltBuild
    let suggestedVersion = view?.suggestedTiltVersion
    const versionSettings = view?.versionSettings
    const checkUpdates = versionSettings?.checkUpdates ?? true
    let shareSnapshotModal = this.renderShareSnapshotModal(view)
    let fatalErrorModal = this.renderFatalErrorModal(view)
    let errorModal = this.renderErrorModal()

    let statusbar = (
      <Statusbar
        items={statusItems}
        alertsUrl={this.path("/alerts")}
        runningBuild={runningBuild}
        suggestedVersion={suggestedVersion}
        checkVersion={checkUpdates}
      />
    )

    let hudClasses = ["HUD"]
    if (this.pathBuilder.isSnapshot()) {
      hudClasses.push("is-snapshot")
    }

    let matchTrace = matchPath(String(this.props.history.location.pathname), {
      path: this.path("/r/:name/trace/:span"),
    })
    let matchTraceParams: any = matchTrace?.params
    let isTwoLevelHeader = !!matchTraceParams?.span
    let matchOverview =
      matchPath(String(this.props.history.location.pathname), {
        path: this.path("/overview"),
      }) ||
      matchPath(String(this.props.history.location.pathname), {
        path: this.path("/r/:name/overview"),
      })

    if (matchOverview) {
      return (
        <LocalStorageContextProvider tiltfileKey={view.tiltfileKey}>
          <div className={hudClasses.join(" ")}>
            <AnalyticsNudge needsNudge={needsNudge} />
            <SocketBar state={this.state.socketState} />
            {fatalErrorModal}
            {errorModal}
            {shareSnapshotModal}

            {this.renderOverviewSwitch()}
          </div>
        </LocalStorageContextProvider>
      )
    }

    return (
      <LocalStorageContextProvider tiltfileKey={view.tiltfileKey}>
        <div className={hudClasses.join(" ")}>
          <AnalyticsNudge needsNudge={needsNudge} />
          <SocketBar state={this.state.socketState} />
          {fatalErrorModal}
          {errorModal}
          {shareSnapshotModal}

          {this.renderSidebarSwitch()}
          {statusbar}

          <HUDLayout
            header={this.renderHUDHeader()}
            isSidebarClosed={!!this.state.isSidebarClosed}
            isTwoLevelHeader={isTwoLevelHeader}
          >
            {this.renderMainPaneSwitch()}
          </HUDLayout>
        </div>
      </LocalStorageContextProvider>
    )
  }

  renderOverviewSwitch() {
    return (
      <PathBuilderProvider value={this.pathBuilder}>
        <LogStoreProvider value={this.state.logStore || new LogStore()}>
          <Switch>
            <Route
              path={this.path("/r/:name/overview")}
              render={(props: RouteComponentProps<any>) => (
                <OverviewResourcePane
                  name={props.match.params?.name || ""}
                  view={this.state.view}
                />
              )}
            />
            <Route render={() => <OverviewPane view={this.state.view} />} />
          </Switch>
        </LogStoreProvider>
      </PathBuilderProvider>
    )
  }

  renderHUDHeader() {
    return (
      <>
        {this.renderResourceInfo()}
        {this.renderSecondaryNav()}
      </>
    )
  }

  renderResourceInfo() {
    let match = matchPath(String(this.props.history.location.pathname), {
      path: this.path("/r/:name"),
    })
    let params: any = match?.params
    let name = params?.name

    let view = this.state.view
    let showCopySuccess = this.state.showCopySuccess
    let resources = view?.resources ?? []
    let selectedResource = resources?.find((r) => r.name === name)

    let endpoints = selectedResource?.endpointLinks ?? []
    let podID = selectedResource?.podID ?? ""
    let podStatus =
      (selectedResource?.k8sResourceInfo &&
        selectedResource?.k8sResourceInfo.podStatus) ||
      ""

    let showSnapshot =
      this.getFeatures().isEnabled("snapshots") &&
      !this.pathBuilder.isSnapshot()
    let snapshotHighlight = this.state.snapshotHighlight || null

    return (
      <ResourceInfo
        endpoints={endpoints}
        podID={podID}
        podStatus={podStatus}
        showSnapshotButton={showSnapshot}
        showCopySuccess={showCopySuccess}
        highlight={snapshotHighlight}
        handleOpenModal={this.handleOpenModal}
        handleClickCopy={this.handleShowCopySuccess}
      />
    )
  }

  renderSecondaryNav() {
    let view = this.state.view
    let resources = view?.resources ?? []
    let hasK8s = resources.some((r) => {
      let specs = r.specs ?? []
      return specs.some((spec) => spec.type === TargetType.K8s)
    })

    let secondaryNavRoute = (
      t: ResourceView,
      props: RouteComponentProps<any>
    ) => {
      let name = props.match.params?.name ?? ""
      let span = props.match.params?.span ?? ""
      let numAlerts = 0
      let logUrl = name === "" ? this.path("/") : this.path(`/r/${name}`)
      let alertsUrl =
        name === "" ? this.path("/alerts") : this.path(`/r/${name}/alerts`)

      let facetsUrl = name !== "" ? this.path(`/r/${name}/facets`) : null
      let metricsUrl = ""
      let isSnapshot = this.pathBuilder.isSnapshot()
      let showMetricsTab =
        name === "" && !isSnapshot && (hasK8s || view?.metricsServing?.mode)
      if (showMetricsTab) {
        metricsUrl = this.path("/metrics")
      }

      let currentTraceNav =
        span && this.state.logStore
          ? traceNav(this.state.logStore, this.pathBuilder, span)
          : null

      if (name) {
        let selectedResource = resources.find((r) => r.name === name)
        if (selectedResource) {
          numAlerts = combinedAlerts(selectedResource, null).length
        }
      } else {
        numAlerts = resources
          .map((r) => combinedAlerts(r, null).length)
          .reduce((sum, current) => sum + current, 0)
      }

      return (
        <SecondaryNav
          logUrl={logUrl}
          alertsUrl={alertsUrl}
          resourceView={t}
          numberOfAlerts={numAlerts}
          facetsUrl={facetsUrl}
          metricsUrl={metricsUrl}
          traceNav={currentTraceNav}
        />
      )
    }

    return (
      <Switch>
        <Route
          path={this.path("/r/:name/alerts")}
          render={secondaryNavRoute.bind(null, ResourceView.Alerts)}
        />
        <Route
          path={this.path("/r/:name/facets")}
          render={secondaryNavRoute.bind(null, ResourceView.Facets)}
        />
        <Route
          path={this.path("/r/:name/trace/:span")}
          render={secondaryNavRoute.bind(null, ResourceView.Trace)}
        />
        <Route
          path={this.path("/r/:name")}
          render={secondaryNavRoute.bind(null, ResourceView.Log)}
        />
        <Route
          path={this.path("/alerts")}
          render={secondaryNavRoute.bind(null, ResourceView.Alerts)}
        />
        <Route
          path={this.path("/metrics")}
          render={secondaryNavRoute.bind(null, ResourceView.Metrics)}
        />
        <Route render={secondaryNavRoute.bind(null, ResourceView.Log)} />
      </Switch>
    )
  }

  renderSidebarSwitch() {
    let view = this.state.view
    let resources = view?.resources || []
    let sidebarItems = resources.map((res) => new SidebarItem(res))
    let isSidebarClosed = !!this.state.isSidebarClosed
    let tiltCloudUsername = view?.tiltCloudUsername || null
    let tiltCloudSchemeHost = view?.tiltCloudSchemeHost || ""
    let tiltCloudTeamID = view?.tiltCloudTeamID || null
    let tiltCloudTeamName = view?.tiltCloudTeamName || null
    let isSnapshot = this.pathBuilder.isSnapshot()
    let sidebarRoute = (t: ResourceView, props: RouteComponentProps<any>) => {
      let name = props.match.params.name
      return (
        <Sidebar isClosed={isSidebarClosed} toggleSidebar={this.toggleSidebar}>
          <SidebarAccount
            tiltCloudUsername={tiltCloudUsername}
            tiltCloudSchemeHost={tiltCloudSchemeHost}
            tiltCloudTeamID={tiltCloudTeamID}
            tiltCloudTeamName={tiltCloudTeamName}
            isSnapshot={isSnapshot}
          />
          <SidebarResources
            selected={name}
            items={sidebarItems}
            resourceView={t}
            pathBuilder={this.pathBuilder}
          />
        </Sidebar>
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
          path={this.path("/metrics")}
          render={sidebarRoute.bind(null, ResourceView.Metrics)}
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
    let isSnapshot = this.pathBuilder.isSnapshot()

    let traceRoute = (props: RouteComponentProps<any>) => {
      let name = props.match.params?.name ?? ""
      let span = props.match.params?.span ?? ""

      let r = resources.find((r) => r.name === name)
      if (r === undefined) {
        return <Route component={NotFound} />
      }

      let logLines: LogLine[] = []
      if (span && logStore) {
        logLines = logStore.traceLog(span)
      }

      return (
        <LogPane
          logLines={logLines}
          showManifestPrefix={false}
          manifestName={name}
          handleSetHighlight={this.handleSetHighlight}
          handleClearHighlight={this.handleClearHighlight}
          highlight={snapshotHighlight}
          isSnapshot={isSnapshot}
        />
      )
    }

    let logsRoute = (props: RouteComponentProps<any>) => {
      let name = props.match.params?.name ?? ""
      let r = resources.find((r) => r.name === name)
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
          manifestName={name}
          handleSetHighlight={this.handleSetHighlight}
          handleClearHighlight={this.handleClearHighlight}
          highlight={snapshotHighlight}
          isSnapshot={isSnapshot}
        />
      )
    }

    let errorRoute = (props: RouteComponentProps<any>): React.ReactNode => {
      let name = props.match.params ? props.match.params.name : ""
      let er = resources.find((r) => r.name === name)
      if (!er) {
        return <Route component={NotFound} />
      }
      return (
        <AlertPane
          pathBuilder={this.pathBuilder}
          resources={[er]}
          logStore={logStore}
        />
      )
    }
    let facetsRoute = (props: RouteComponentProps<any>): React.ReactNode => {
      let name = props.match.params ? props.match.params.name : ""
      let fr = resources.find((r) => r.name === name)
      if (!fr) {
        return <Route component={NotFound} />
      }
      return <FacetsPane resource={fr} logStore={logStore} />
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
          manifestName={""}
          handleSetHighlight={this.handleSetHighlight}
          handleClearHighlight={this.handleClearHighlight}
          highlight={this.state.snapshotHighlight}
          isSnapshot={isSnapshot}
        />
      )
    }

    let serving = view.metricsServing as Proto.webviewMetricsServing

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
        <Route
          exact
          path={this.path("/metrics")}
          render={() => (
            <MetricsPane pathBuilder={this.pathBuilder} serving={serving} />
          )}
        />
        <Route exact path={this.path("/r/:name")} render={logsRoute} />
        <Route
          exact
          path={this.path("/r/:name/trace/:span")}
          render={traceRoute}
        />
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

  renderErrorModal() {
    return (
      <ErrorModal
        error={this.state.error}
        showErrorModal={this.state.showErrorModal}
        handleClose={() =>
          this.setState({
            showErrorModal: ShowErrorModal.Default,
            error: undefined,
          })
        }
      />
    )
  }
}

export default HUD
