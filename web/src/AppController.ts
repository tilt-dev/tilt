import HudState from "./HudState"
import PathBuilder from "./PathBuilder"
import { Snapshot, SocketState } from "./types"

interface HudInt {
  onAppChange: <K extends keyof HudState>(state: Pick<HudState, K>) => void
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
   *     Has one method, onAppChange, that receives all the updates
   *     for the application.
   */
  constructor(pathBuilder: PathBuilder, component: HudInt) {
    if (!component.onAppChange) {
      throw new Error("App component has no onAppChange method")
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
    fetch("/api/websocket_token")
      .then((res) => res.text())
      .then((text) => {
        this.socket = new WebSocket(`${this.url}?csrf=${text}`)
        let socket = this.socket

        this.socket.addEventListener("close", this.onSocketClose.bind(this))
        this.socket.addEventListener("message", (event) => {
          if (!this.liveSocket) {
            this.loadCount++
          }
          this.liveSocket = true
          this.tryConnectCount = 0

          let data: Proto.webviewView = JSON.parse(event.data)

          // @ts-ignore
          this.component.onAppChange({
            view: data,
            socketState: SocketState.Active,
          })
        })
      })
      .catch((err) => {
        console.error("fetching websocket token: " + err)
        this.onSocketClose()
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
      this.component.onAppChange({
        socketState: SocketState.Closed,
      })
      this.createNewSocket()
      return
    }

    let backoff = Math.pow(2, this.tryConnectCount) * 1000
    let maxTimeout = 10 * 1000 // 10sec
    let isLocal =
      this.url.indexOf("ws://localhost") === 0 ||
      this.url.indexOf("wss://localhost") === 0
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
      let state: SocketState =
        this.loadCount || this.tryConnectCount > 1
          ? SocketState.Reconnecting
          : SocketState.Loading
      this.component.onAppChange({
        socketState: state,
      })
      this.createNewSocket()
    }, timeout)
  }

  setStateFromSnapshot(): void {
    let url = this.url
    fetch(url)
      .then((resp) => resp.json())
      .then((data: Snapshot) => {
        data.view = data.view || {}

        this.component.onAppChange({
          view: data.view,
        })
        if (data.path) {
          this.component.setHistoryLocation(this.pb.path(data.path))
        }
      })
      .catch((err) => {
        console.error(err)
        this.component.onAppChange({ error: err })
      })
  }
}

export default AppController
