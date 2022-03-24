import React, { useContext } from "react"

// A little helper class for building paths relative to the root of the app.
class PathBuilder {
  private protocol: string
  private host: string
  private snapId: string = ""

  static forTesting(host: string, pathname: string) {
    return new PathBuilder({
      protocol: "http:",
      host: host,
      pathname: pathname,
    })
  }

  constructor(loc: { protocol: string; host: string; pathname: string }) {
    this.host = loc.host
    this.protocol = loc.protocol

    const snapshotRe = new RegExp("^/snapshot/([^/]+)")
    let snapMatch = snapshotRe.exec(loc.pathname)
    if (snapMatch) {
      this.snapId = snapMatch[1]
    }
  }

  getDataUrl() {
    if (this.isSnapshot()) {
      return this.snapshotDataUrl()
    }
    return this.isSecure()
      ? `wss://${this.host}/ws/view`
      : `ws://${this.host}/ws/view`
  }

  isSecure(): boolean {
    return this.protocol === "https:"
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

  path(relPath: string): string {
    if (relPath[0] !== "/") {
      throw new Error('relPath should start with "/", actual:' + relPath)
    }
    return this.rootPath() + relPath
  }

  // A template literal function for encoding paths.
  // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Template_literals
  encpath(strings: TemplateStringsArray, ...values: string[]): string {
    let result = [strings[0]]
    for (let i = 0; i < values.length; i++) {
      result.push(encodeURIComponent(values[i]), strings[i + 1])
    }
    return this.path(result.join(""))
  }
}

export default PathBuilder

export const pathBuilderContext = React.createContext<PathBuilder>(
  new PathBuilder(window.location)
)

export function usePathBuilder(): PathBuilder {
  return useContext(pathBuilderContext)
}
export const PathBuilderProvider = pathBuilderContext.Provider
