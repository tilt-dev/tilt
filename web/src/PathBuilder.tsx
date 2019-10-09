// A little helper class for building paths relative to the root of the app.

class PathBuilder {
  private host: string
  private roomId: string = ""
  private snapId: string = ""

  constructor(host: string, pathname: string) {
    this.host = host

    const roomRe = new RegExp("^/view/([^/]+)")
    let roomMatch = roomRe.exec(pathname)
    if (roomMatch) {
      this.roomId = roomMatch[1]
    }
    const snapshotRe = new RegExp("^/snapshot/([^/]+)")
    let snapMatch = snapshotRe.exec(pathname)
    if (snapMatch) {
      this.snapId = snapMatch[1]
    }
  }

  getDataUrl() {
    let scheme = "wss"
    if (this.isLocal()) {
      scheme = "ws"
    }
    if (this.isSnapshot()) {
      return this.snapshotDataUrl()
    }
    if (this.roomId) {
      return `${scheme}://${this.host}/join/${this.roomId}`
    }
    return `${scheme}://${this.host}/ws/view`
  }

  isLocal() {
    return this.host.indexOf("localhost") === 0
  }

  isSnapshot(): boolean {
    return this.snapId !== ""
  }

  private snapshotDataUrl(): string {
    let scheme = "https"
    if (this.isLocal()) {
      scheme = "http"
    }
    return `${scheme}://${this.host}/api/snapshot/${this.snapId}`
  }

  private snapshotPathBase(): string {
    return `/snapshot/${this.snapId}`
  }

  rootPath() {
    if (this.roomId) {
      return `/view/${this.roomId}`
    }
    if (this.isSnapshot()) {
      return this.snapshotPathBase()
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
