import { SocketState, Snapshot } from "./types"
import HudState from "./HudState"
import PathBuilder from "./PathBuilder"

type Resource = Proto.webviewResource
type K8sResourceInfo = Proto.webviewK8sResourceInfo

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
    let socket = this.socket

    this.socket.addEventListener("close", this.onSocketClose.bind(this))
    this.socket.addEventListener("message", event => {
      if (!this.liveSocket) {
        this.loadCount++
      }
      this.liveSocket = true
      this.tryConnectCount = 0

      let data: Proto.webviewView = JSON.parse(event.data)
      let toCheckpoint = data.logList?.toCheckpoint
      if (toCheckpoint && toCheckpoint > 0) {
        let tiltStartTime = data.tiltStartTime
        let response: Proto.webviewAckWebsocketRequest = {
          toCheckpoint,
          tiltStartTime,
        }
        socket.send(JSON.stringify(response))
      }

      // @ts-ignore
      this.component.setAppState({
        view: data,
        socketState: SocketState.Active,
      })
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
        data.view = data.view || {}

        this.component.setAppState({
          view: data.view,
          isSidebarClosed: data.isSidebarClosed,
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
