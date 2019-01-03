// A Websocket that automatically retries.

class AppController {
  constructor(url, component) {
    this.url = url
    this.component = component
    this.tryConnectCount = 0
    this.liveSocket = false
    this.loadCount = 0
    this.createNewSocket()
  }

  createNewSocket() {
    this.tryConnectCount++
    this.socket = new WebSocket(this.url)
    this.socket.addEventListener('close', this.onSocketClose.bind(this))
    this.socket.addEventListener('message', (event) => {
      if (!this.liveSocket) {
        this.loadCount++
      }
      this.liveSocket = true
      this.tryConnectCount = 0

      let data = JSON.parse(event.data)
      this.component.setState({View: data})
    })
  }

  onSocketClose() {
    let wasAlive = this.liveSocket
    this.liveSocket = false

    if (wasAlive) {
      this.component.setState({View: null, Message: 'Disconnected…'})
      this.createNewSocket()
      return
    }

    let timeout = Math.pow(2, this.tryConnectCount) * 1000
    let maxTimeout = 5 * 1000 // 5sec
    setTimeout(() => {
      let message = this.loadCount ? 'Reconnecting…' : 'Loading…'
      this.component.setState({View: null, Message: message})
      this.createNewSocket()
    }, Math.min(maxTimeout, timeout))
  }
}

export default AppController
