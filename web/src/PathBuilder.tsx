// A little helper class for building paths relative to the root of the app.

class PathBuilder {
  private host: string
  private roomId: string

  constructor(host: string, pathname: string) {
    this.host = host
    this.roomId = ""

    let roomRe = new RegExp("^/view/([^/]+)")
    let roomMatch = roomRe.exec(pathname)
    if (roomMatch) {
      this.roomId = roomMatch[1]
    }
  }

  getWebsocketUrl() {
    let scheme = "wss"
    if (this.isLocal()) {
      scheme = "ws"
    }
    if (this.roomId) {
      return `${scheme}://${this.host}/join/${this.roomId}`
    }
    return `${scheme}://${this.host}/ws/view`
  }

  isLocal() {
    return this.host.indexOf("localhost") == 0
  }

  rootPath() {
    if (this.roomId) {
      return `/view/${this.roomId}`
    }
    return ""
  }

  path(relPath: string) {
    if (relPath[0] !== "/") {
      throw new Error('relPath should start with "/", actual:' + relPath)
    }
    return this.rootPath() + relPath
  }
}

export default PathBuilder
