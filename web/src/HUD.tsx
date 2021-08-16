import { StylesProvider } from "@material-ui/core/styles"
import { History, UnregisterCallback } from "history"
import React, { Component } from "react"
import ReactOutlineManager from "react-outline-manager"
import { useHistory } from "react-router"
import { Route, RouteComponentProps, Switch } from "react-router-dom"
import { incr, navigationToTags } from "./analytics"
import AnalyticsNudge from "./AnalyticsNudge"
import AppController from "./AppController"
import ErrorModal from "./ErrorModal"
import FatalErrorModal from "./FatalErrorModal"
import Features, { FeaturesProvider, Flag } from "./feature"
import HeroScreen from "./HeroScreen"
import "./HUD.scss"
import HudState from "./HudState"
import { InterfaceVersion, useInterfaceVersion } from "./InterfaceVersion"
import { tiltfileKeyContext } from "./LocalStorage"
import LogStore, { LogStoreProvider } from "./LogStore"
import OverviewResourcePane from "./OverviewResourcePane"
import OverviewTablePane from "./OverviewTablePane"
import PathBuilder, { PathBuilderProvider } from "./PathBuilder"
import { ResourceNavProvider } from "./ResourceNav"
import ShareSnapshotModal from "./ShareSnapshotModal"
import { SnapshotActionProvider } from "./snapshot"
import SocketBar from "./SocketBar"
import { StarredResourcesContextProvider } from "./StarredResourcesContext"
import { ShowErrorModal, ShowFatalErrorModal, SocketState } from "./types"

type HudProps = {
  history: History
  interfaceVersion: InterfaceVersion
}

// Snapshot logs are capped to 1MB (max upload size is 4MB; this ensures between the rest of state and JSON overhead
// that the snapshot should still fit)
const maxSnapshotLogSize = 1000 * 1000

// The Main HUD view, as specified in
// https://docs.google.com/document/d/1VNIGfpC4fMfkscboW0bjYYFJl07um_1tsFrbN-Fu3FI/edit#heading=h.l8mmnclsuxl1
export default class HUD extends Component<HudProps, HudState> {
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
    this.unlisten = this.history.listen((location, action) => {
      let tags = navigationToTags(location, action)
      incr("ui.web.navigation", tags)
    })

    this.state = {
      view: {},
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

  onAppChange<K extends keyof HudState>(stateUpdates: Pick<HudState, K>) {
    this.setState((prevState) => mergeAppUpdate(prevState, stateUpdates))
  }

  setHistoryLocation(path: string) {
    this.props.history.replace(path)
  }

  path(relPath: string) {
    return this.pathBuilder.path(relPath)
  }

  snapshotFromState(state: HudState): Proto.webviewSnapshot {
    let view: any = {}
    if (state.view) {
      Object.assign(view, state.view)
    }
    if (state.logStore) {
      view.logList = state.logStore.toLogList(maxSnapshotLogSize)
    }
    return {
      view: view,
      path: this.props.history.location.pathname,
      snapshotHighlight: state.snapshotHighlight,
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
            this.setState({
              showSnapshotModal: false,
              error: "Error decoding JSON response",
            })
          })
      })
      .catch((err) => {
        console.error(err)
        this.setState({
          showSnapshotModal: false,
          error: "Error posting snapshot",
        })
      })
  }

  private getFeatures(): Features {
    let featureFlags = {} as { [key: string]: boolean }
    let flagList = this.state.view.uiSession?.status?.featureFlags || []
    flagList.forEach((flag) => {
      featureFlags[flag.name || ""] = !!flag.value
    })
    return new Features(featureFlags)
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
    let session = this.state.view.uiSession?.status

    let needsNudge = session?.needsAnalyticsNudge ?? false
    let resources = view?.uiResources ?? []
    if (!resources?.length || !session?.tiltfileKey) {
      return <HeroScreen message={"Loading…"} />
    }

    let tiltfileKey = session?.tiltfileKey
    let runningBuild = session?.runningTiltBuild
    let suggestedVersion = session?.suggestedTiltVersion
    const versionSettings = session?.versionSettings
    const checkUpdates = versionSettings?.checkUpdates ?? true
    let shareSnapshotModal = this.renderShareSnapshotModal(view)
    let fatalErrorModal = this.renderFatalErrorModal(view)
    let errorModal = this.renderErrorModal()

    let hudClasses = ["HUD"]
    if (this.pathBuilder.isSnapshot()) {
      hudClasses.push("is-snapshot")
    }

    let validateResource = (name: string) =>
      resources.some((res) => res.metadata?.name === name)
    return (
      <tiltfileKeyContext.Provider value={tiltfileKey}>
        <StarredResourcesContextProvider>
          <ReactOutlineManager>
            <ResourceNavProvider validateResource={validateResource}>
              <div className={hudClasses.join(" ")}>
                <AnalyticsNudge needsNudge={needsNudge} />
                <SocketBar state={this.state.socketState} />
                {fatalErrorModal}
                {errorModal}
                {shareSnapshotModal}

                {this.renderOverviewSwitch()}
              </div>
            </ResourceNavProvider>
          </ReactOutlineManager>
        </StarredResourcesContextProvider>
      </tiltfileKeyContext.Provider>
    )
  }

  renderOverviewSwitch() {
    const features = this.getFeatures()
    let showSnapshot =
      features.isEnabled(Flag.Snapshots) && !this.pathBuilder.isSnapshot()
    let snapshotAction = {
      enabled: showSnapshot,
      openModal: this.handleOpenModal,
    }

    return (
      /* allow Styled Components to override MUI - https://material-ui.com/guides/interoperability/#controlling-priority-3*/
      <StylesProvider injectFirst>
        <FeaturesProvider value={features}>
          <SnapshotActionProvider value={snapshotAction}>
            <PathBuilderProvider value={this.pathBuilder}>
              <LogStoreProvider value={this.state.logStore || new LogStore()}>
                <Switch>
                  <Route
                    path={this.path("/r/:name/overview")}
                    render={(props: RouteComponentProps<any>) => (
                      <OverviewResourcePane view={this.state.view} />
                    )}
                  />
                  <Route
                    render={() => <OverviewTablePane view={this.state.view} />}
                  />
                </Switch>
              </LogStoreProvider>
            </PathBuilderProvider>
          </SnapshotActionProvider>
        </FeaturesProvider>
      </StylesProvider>
    )
  }

  renderShareSnapshotModal(view: Proto.webviewView | null) {
    let handleClose = () =>
      this.setState({ showSnapshotModal: false, snapshotLink: "" })
    let handleSendSnapshot = () =>
      this.sendSnapshot(this.snapshotFromState(this.state))
    let session = view?.uiSession?.status
    let tiltCloudUsername = session?.tiltCloudUsername || null
    let tiltCloudSchemeHost = session?.tiltCloudSchemeHost || ""
    let tiltCloudTeamID = session?.tiltCloudTeamID || null
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
    let session = view?.uiSession?.status
    let error = session?.fatalError
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

export function HUDFromContext(props: React.PropsWithChildren<{}>) {
  let history = useHistory()
  let interfaceVersion = useInterfaceVersion()
  return <HUD history={history} interfaceVersion={interfaceVersion} />
}

function compareObjectsOrder<
  T extends { status?: any; metadata?: Proto.v1ObjectMeta }
>(a: T, b: T): number {
  let aStatus = a.status as Proto.v1alpha1UIResourceStatus | null
  let bStatus = b.status as Proto.v1alpha1UIResourceStatus | null
  let aOrder = aStatus?.order || 0
  let bOrder = bStatus?.order || 0
  if (aOrder != bOrder) {
    return aOrder - bOrder
  }

  let aName = a.metadata?.name || ""
  let bName = a.metadata?.name || ""
  return aName < bName ? -1 : aName == bName ? 0 : 1
}

// returns a copy of `prev` that has the adds/updates/deletes from `updates` applied
function mergeObjectUpdates<T extends { metadata?: Proto.v1ObjectMeta }>(
  updates: T[] | undefined,
  prev: T[] | undefined
): T[] {
  let next = Array.from(prev || [])
  if (updates) {
    updates.forEach((u) => {
      let index = next.findIndex((o) => o?.metadata?.name === u?.metadata?.name)
      if (index === -1) {
        next.push(u)
      } else {
        next[index] = u
      }
    })
    next = next.filter((o) => !o?.metadata?.deletionTimestamp)
  }

  next.sort(compareObjectsOrder)

  return next
}

export function mergeAppUpdate<K extends keyof HudState>(
  prevState: Readonly<HudState>,
  stateUpdates: Pick<HudState, K>
): null | Pick<HudState, K> {
  // All fields are optional on a HudState, so it's ok to pretent
  // a Pick<HudState> and a HudState are the same.
  let state = stateUpdates as HudState

  let oldStartTime = prevState.view?.tiltStartTime
  let newStartTime = state.view?.tiltStartTime
  if (oldStartTime && newStartTime && oldStartTime != newStartTime) {
    // If Tilt restarts, reload the page to get new JS.
    // https://github.com/tilt-dev/tilt/issues/4421
    window.location.reload()
    return prevState
  }

  let logListUpdate = state.view?.logList
  if (state.view?.isComplete) {
    // If this is a full state refresh, replace the view field
    // and the log store completely.
    let newState = { ...state } as any
    newState.view = state.view
    newState.logStore = new LogStore()
    newState.logStore.append(logListUpdate)
    return newState
  }

  // Otherwise, merge the new state updates into the old state object.
  let result = { ...state }

  // We're going to merge in view updates manually.
  result.view = prevState.view

  if (logListUpdate) {
    // We can assume state always has a log store.
    prevState.logStore!.append(logListUpdate)
  }

  // Merge the UISession
  let sessionUpdate = state.view?.uiSession
  if (sessionUpdate) {
    result.view = Object.assign({}, result.view, {
      uiSession: sessionUpdate,
    })
  }

  const uiResourceUpdates = state.view?.uiResources
  if (uiResourceUpdates) {
    result.view = Object.assign({}, result.view, {
      uiResources: mergeObjectUpdates(
        uiResourceUpdates,
        result.view?.uiResources
      ),
    })
  }

  const uiButtonUpdates = state.view?.uiButtons
  if (uiButtonUpdates) {
    result.view = Object.assign({}, result.view, {
      uiButtons: mergeObjectUpdates(uiButtonUpdates, result.view?.uiButtons),
    })
  }

  // If no references have changed, don't re-render.
  //
  // LogStore handles its own update events, so a change
  // to LogStore doesn't update its reference.
  // This makes rendering much, much faster for apps
  // with lots of logs.
  if (!hasChange(result, prevState)) {
    return null
  }

  return result
}

function hasChange(result: any, prevState: any): boolean {
  for (let k in result) {
    let resultV = result[k] as any
    let prevV = prevState[k] as any
    if (resultV !== prevV) {
      return true
    }
  }
  return false
}
