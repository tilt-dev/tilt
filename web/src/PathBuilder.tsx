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
    if (this.roomId) {
      return `ws://${this.host}/join/${this.roomId}`
    }
    return `ws://${this.host}/ws/view`
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
