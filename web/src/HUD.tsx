import { History, UnregisterCallback } from "history"
import React, { Component } from "react"
import { useHistory } from "react-router"
import { Route, RouteComponentProps, Switch } from "react-router-dom"
import { incr, navigationToTags } from "./analytics"
import AnalyticsNudge from "./AnalyticsNudge"
import AppController from "./AppController"
import ErrorModal from "./ErrorModal"
import FatalErrorModal from "./FatalErrorModal"
import Features from "./feature"
import HeroScreen from "./HeroScreen"
import "./HUD.scss"
import HudState from "./HudState"
import { InterfaceVersion, useInterfaceVersion } from "./InterfaceVersion"
import { tiltfileKeyContext } from "./LocalStorage"
import LogStore, { LogStoreProvider } from "./LogStore"
import OverviewPane from "./OverviewPane"
import OverviewResourcePane from "./OverviewResourcePane"
import PathBuilder, { PathBuilderProvider } from "./PathBuilder"
import ShareSnapshotModal from "./ShareSnapshotModal"
import { SidebarPinContextProvider } from "./SidebarPin"
import { SnapshotActionProvider } from "./snapshot"
import SocketBar from "./SocketBar"
import { StatusItem } from "./Statusbar"
import { OverviewNavProvider } from "./TabNav"
import {
  ShowErrorModal,
  ShowFatalErrorModal,
  SnapshotHighlight,
  SocketState,
} from "./types"

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
    this.unlisten = this.history.listen((location: any, action: string) => {
      let tags = navigationToTags(location, action)
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
      let newState: any = {}
      Object.assign(newState, state)
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

    let hudClasses = ["HUD"]
    if (this.pathBuilder.isSnapshot()) {
      hudClasses.push("is-snapshot")
    }

    let validateTab = (name: string) =>
      resources.some((res) => res.name === name)
    return (
      <tiltfileKeyContext.Provider value={view.tiltfileKey}>
        <SidebarPinContextProvider>
          <OverviewNavProvider validateTab={validateTab}>
            <div className={hudClasses.join(" ")}>
              <AnalyticsNudge needsNudge={needsNudge} />
              <SocketBar state={this.state.socketState} />
              {fatalErrorModal}
              {errorModal}
              {shareSnapshotModal}

              {this.renderOverviewSwitch()}
            </div>
          </OverviewNavProvider>
        </SidebarPinContextProvider>
      </tiltfileKeyContext.Provider>
    )
  }

  renderOverviewSwitch() {
    let showSnapshot =
      this.getFeatures().isEnabled("snapshots") &&
      !this.pathBuilder.isSnapshot()
    let snapshotAction = {
      enabled: showSnapshot,
      openModal: this.handleOpenModal,
    }

    return (
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
              <Route render={() => <OverviewPane view={this.state.view} />} />
            </Switch>
          </LogStoreProvider>
        </PathBuilderProvider>
      </SnapshotActionProvider>
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

export function HUDFromContext(props: React.PropsWithChildren<{}>) {
  let history = useHistory()
  let interfaceVersion = useInterfaceVersion()
  return <HUD history={history} interfaceVersion={interfaceVersion} />
}
