import { getResourceAlerts } from "./alerts"
import {
  K8sResourceInfo,
  SocketState,
  WebView,
  Snapshot,
  Resource,
  HudState,
} from "./types"
import PathBuilder from "./PathBuilder"

interface HudInt {
  setAppState: <K extends keyof HudState>(state: Pick<HudState, K>) => void
  setHistoryLocation: (path: string) => void
}

// A Websocket that automatically retries.
class AppController {
  url: string
  loadCount: number
  liveSocket: boolean
  tryConnectCount: number
  socket: WebSocket | null = null
  component: HudInt
  disposed: boolean = false
  pb: PathBuilder

  /**
   * @param pathBuilder a PathBuilder
   * @param component The top-level component for the app.
   *     Has one method, setAppState, that sets the global state of the
   *     app.
   */
  constructor(pathBuilder: PathBuilder, component: HudInt) {
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

      let data: WebView = JSON.parse(event.data)

      data.Resources = this.setDefaultResourceInfo(data.Resources)
      // @ts-ignore
      this.component.setAppState({
        View: data,
        socketState: SocketState.Active,
      })
    })
  }

  setDefaultResourceInfo(resources: Array<Resource>): Array<Resource> {
    return resources.map(r => {
      if (!r.K8sResourceInfo && !r.DCResourceInfo) {
        let ri: K8sResourceInfo = {
          PodName: "",
          PodCreationTime: "",
          PodUpdateStartTime: "",
          PodStatus: "",
          PodStatusMessage: "",
          PodRestarts: 0,
          PodLog: "",
        }
        r.K8sResourceInfo = ri
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
        socketState: SocketState.Closed,
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
      let state: SocketState = this.loadCount
        ? SocketState.Reconnecting
        : SocketState.Loading
      this.component.setAppState({
        socketState: state,
      })
      this.createNewSocket()
    }, timeout)
  }

  setStateFromSnapshot(): void {
    let url = this.url
    fetch(url)
      .then(resp => resp.json())
      .then((data: Snapshot) => {
        data.View = data.View || {}

        let resources = (data.View && data.View.Resources) || []
        data.View.Resources = this.setDefaultResourceInfo(resources)

        this.component.setAppState({
          View: data.View,
          IsSidebarClosed: data.IsSidebarClosed,
        })
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
