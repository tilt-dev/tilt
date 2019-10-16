import { getResourceAlerts } from "./alerts"
import { ShowFatalErrorModal } from "./types"
import PathBuilder from "./PathBuilder"

interface HUDInt {
  setAppState: (state: any) => void
  setHistoryLocation: (path: string) => void
}

// A Websocket that automatically retries.
class AppController {
  url: string
  loadCount: number
  liveSocket: boolean
  tryConnectCount: number
  socket: WebSocket | null = null
  component: HUDInt
  disposed: boolean = false
  pb: PathBuilder

  /**
   * @param pathBuilder a PathBuilder
   * @param component The top-level component for the app.
   *     Has one method, setAppState, that sets the global state of the
   *     app. This state has two properties
   *     - Message (string): A status message about the state of the socket
   *     - View (Object): A JSON serialization of the Go struct in internal/renderer/view
   */
  constructor(pathBuilder: PathBuilder, component: HUDInt) {
    if (!component.setAppState) {
      throw new Error("App component has no setAppState method")
    }

    this.pb = pathBuilder
    this.url = pathBuilder.getDataUrl()
    this.component = component
    this.tryConnectCount = 0
    this.liveSocket = false
    this.loadCount = 0
  }

  createNewSocket() {
    this.tryConnectCount++
    this.socket = new WebSocket(this.url)
    this.socket.addEventListener("close", this.onSocketClose.bind(this))
    this.socket.addEventListener("message", event => {
      if (!this.liveSocket) {
        this.loadCount++
      }
      this.liveSocket = true
      this.tryConnectCount = 0

      let data = JSON.parse(event.data)

      data.Resources = this.setDefaultResourceInfo(data.Resources)
      // @ts-ignore
      this.component.setAppState({ View: data })
    })
  }

  setDefaultResourceInfo(resources: Array<any>): Array<any> {
    return resources.map(r => {
      if (!r.K8sResourceInfo && !r.DCResourceInfo) {
        r.K8sResourceInfo = {
          PodName: "",
          PodCreationTime: "",
          PodUpdateStartTime: "",
          PodStatus: "",
          PodStatusMessage: "",
          PodRestarts: 0,
          PodLog: "",
          Endpoints: [],
        }
      }
      r.Alerts = getResourceAlerts(r)
      return r
    })
  }

  dispose() {
    this.disposed = true
    if (this.socket) {
      this.socket.close()
    }
  }

  onSocketClose() {
    let wasAlive = this.liveSocket
    this.liveSocket = false
    if (this.disposed) {
      return
    }

    if (wasAlive) {
      this.component.setAppState({
        View: null,
        Message: "Disconnected…",
        IsSidebarClosed: false,
        SnapshotLink: "",
        showSnapshotModal: false,
        showFatalErrorModal: ShowFatalErrorModal.Default,
        snapshotHighlight: null,
      })
      this.createNewSocket()
      return
    }

    let backoff = Math.pow(2, this.tryConnectCount) * 1000
    let maxTimeout = 10 * 1000 // 10sec
    let isLocal = this.url.indexOf("ws://localhost") === 0
    if (isLocal) {
      // if this is a local connection, max out at 1.5sec.
      // this makes it a bit easier to detect when a window is already open.
      maxTimeout = 1500
    }
    let timeout = Math.min(maxTimeout, backoff)

    setTimeout(() => {
      if (this.disposed) {
        return
      }
      let message = this.loadCount ? "Reconnecting…" : "Loading…"
      this.component.setAppState({
        View: null,
        Message: message,
        IsSidebarClosed: false,
        SnapshotLink: "",
        showSnapshotModal: false,
        showFatalErrorModal: ShowFatalErrorModal.Default,
        snapshotHighlight: null,
      })
      this.createNewSocket()
    }, timeout)
  }

  setStateFromSnapshot(): void {
    let url = this.url
    fetch(url)
      .then(resp => resp.json())
      .then(data => {
        data.View.Resources = this.setDefaultResourceInfo(data.View.Resources)
        // @ts-ignore
        this.component.setAppState({ View: data.View })
        if (data.path) {
          this.component.setHistoryLocation(this.pb.path(data.path))
        }
        if (data.snapshotHighlight) {
          this.component.setAppState({
            snapshotHighlight: data.snapshotHighlight,
          })
        }
      })
      .catch(err => {
        // TODO(dmiller): set app state with an error message
        console.error(err)
      })
  }
}

export default AppController
