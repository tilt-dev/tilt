// A little helper class for building paths relative to the root of the app.

class PathBuilder {
  private host: string
  private snapId: string = ""

  constructor(host: string, pathname: string) {
    this.host = host

    const snapshotRe = new RegExp("^/snapshot/([^/]+)")
    let snapMatch = snapshotRe.exec(pathname)
    if (snapMatch) {
      this.snapId = snapMatch[1]
    }
  }

  getDataUrl() {
    if (this.isSnapshot()) {
      return this.snapshotDataUrl()
    }
    return `ws://${this.host}/ws/view`
  }

  isSnapshot(): boolean {
    return this.snapId !== ""
  }

  private snapshotDataUrl(): string {
    return `/api/snapshot/${this.snapId}`
  }

  private snapshotPathBase(): string {
    return `/snapshot/${this.snapId}`
  }

  rootPath() {
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
