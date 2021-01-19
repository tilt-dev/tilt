import Anser from "anser"
import React, { Component } from "react"
import styled, { keyframes } from "styled-components"
import "./LogPane.scss"
import "./LogPaneLine.scss"
import LogStore, { useLogStore } from "./LogStore"
import PathBuilder, { usePathBuilder } from "./PathBuilder"
import { RafContext, useRaf } from "./raf"
import { Color, SizeUnit } from "./style-helpers"
import { LogLine } from "./types"

type OverviewLogComponentProps = {
  manifestName: string
  pathBuilder: PathBuilder
  logStore: LogStore
  raf: RafContext
  hideBuildLog?: boolean
  hideRunLog?: boolean
}

let LogPaneRoot = styled.section`
  padding: ${SizeUnit(0.25)} 0;
  background-color: ${Color.grayDarkest};
  width: 100%;
  height: 100%;
  overflow-y: auto;
  box-sizing: border-box;
`

const blink = keyframes`
0% {
  opacity: 1;
}
50% {
  opacity: 0;
}
100% {
  opacity: 1;
}
`

let LogEnd = styled.div`
  animation: ${blink} 1s infinite;
  animation-timing-function: ease;
  padding-top: ${SizeUnit(0.25)};
  padding-left: ${SizeUnit(0.625)};
`

let anser = new Anser()

function newLineEl(
  line: LogLine,
  isContextChange: boolean,
  showManifestPrefix: boolean
): Element {
  let text = line.text
  let level = line.level
  let buildEvent = line.buildEvent
  let classes = ["LogPaneLine"]
  if (level === "WARN") {
    classes.push("is-warning")
  } else if (level === "ERROR") {
    classes.push("is-error")
  }
  if (isContextChange) {
    classes.push("is-contextChange")
  }
  if (buildEvent === "init") {
    classes.push("is-buildEvent-init")
  }
  if (buildEvent === "fallback") {
    classes.push("is-buildEvent-fallback")
  }
  let span = document.createElement("span")
  span.classList.add(...classes)

  if (showManifestPrefix) {
    let prefix = document.createElement("span")
    let name = line.manifestName
    if (!name) {
      name = "(global)"
    }
    prefix.title = name
    prefix.className = "logLinePrefix"
    prefix.innerHTML = anser.escapeForHtml(name)
    span.appendChild(prefix)
  }

  let code = document.createElement("code")
  code.classList.add("LogPaneLine-content")

  // newline ensures this takes up at least one line
  let spacer = "\n"
  code.innerHTML = anser.linkify(
    anser.ansiToHtml(anser.escapeForHtml(line.text) + spacer, {
      use_classes: true,
    })
  )
  span.appendChild(code)
  return span
}

// React is not a great system for rendering logs.
// React has to build a virtual DOM, diffs the virtual DOM, and does
// spot updates of the actual DOM.
//
// But logs are append-only, so this wastes a lot of CPU doing diffs
// for things that never change. Other components (like xtermjs) manage
// rendering directly, but have a thin React wrapper to mount the component.
// So we use that rendering strategy here.
//
// This means that we can't use other react components (like styled-components)
// and have to use plain css + HTML.
export class OverviewLogComponent extends Component<OverviewLogComponentProps> {
  autoscroll: boolean = true

  // The element containing all the log lines.
  rootRef: React.RefObject<any> = React.createRef()

  // The blinking cursor at the end fo the component.
  private cursorRef: React.RefObject<HTMLParagraphElement> = React.createRef()

  // Track the scrollTop of the root element to see if the user is scrolling upwards.
  scrollTop: number = -1

  // Timer for tracking autoscroll.
  autoscrollRafId: number | null = null

  private logCheckpoint: number = 0

  private elByStoredLineIndex: { [key: number]: Element } = {}

  constructor(props: OverviewLogComponentProps) {
    super(props)

    this.onScroll = this.onScroll.bind(this)
    this.onLogUpdate = this.onLogUpdate.bind(this)
  }

  scrollCursorIntoView() {
    if (this.cursorRef.current?.scrollIntoView) {
      this.cursorRef.current.scrollIntoView()
    }
  }

  onLogUpdate() {
    if (!this.rootRef.current || !this.cursorRef.current) {
      return
    }

    this.incrementalRender()
  }

  componentDidUpdate(prevProps: OverviewLogComponentProps) {
    if (prevProps.logStore !== this.props.logStore) {
      prevProps.logStore.removeUpdateListener(this.onLogUpdate)
      this.props.logStore.addUpdateListener(this.onLogUpdate)
    }

    if (
      prevProps.manifestName !== this.props.manifestName ||
      prevProps.hideBuildLog !== this.props.hideBuildLog ||
      prevProps.hideRunLog !== this.props.hideRunLog
    ) {
      this.resetRender()
      this.autoscroll = true
      this.scrollTop = -1
      if (this.props.pathBuilder.isSnapshot()) {
        this.autoscroll = false
      }

      this.incrementalRender()
    } else if (prevProps.logStore !== this.props.logStore) {
      this.resetRender()
      this.incrementalRender()
    }
  }

  componentDidMount() {
    let rootEl = this.rootRef.current
    if (!rootEl) {
      return
    }

    if (this.props.pathBuilder.isSnapshot()) {
      this.autoscroll = false
    }

    rootEl.addEventListener("scroll", this.onScroll, {
      passive: true,
    })
    this.resetRender()
    this.incrementalRender()

    this.props.logStore.addUpdateListener(this.onLogUpdate)
  }

  componentWillUnmount() {
    this.props.logStore.removeUpdateListener(this.onLogUpdate)

    let rootEl = this.rootRef.current
    if (!rootEl) {
      return
    }
    rootEl.removeEventListener("scroll", this.onScroll)

    if (this.autoscrollRafId) {
      this.props.raf.cancelAnimationFrame(this.autoscrollRafId)
    }
  }

  onScroll() {
    let rootEl = this.rootRef.current
    if (!rootEl) {
      return
    }

    let scrollTop = rootEl.scrollTop
    let oldScrollTop = this.scrollTop
    let autoscroll = this.autoscroll

    this.scrollTop = scrollTop
    if (oldScrollTop === -1 || oldScrollTop === scrollTop) {
      return
    }

    // If we're scrolled horizontally, cancel the autoscroll.
    if (rootEl.scrollLeft > 0) {
      this.autoscroll = false
      return
    }

    // If we're autoscrolling, and the user scrolled up,
    // cancel the autoscroll.
    if (autoscroll && scrollTop < oldScrollTop) {
      this.autoscroll = false
      return
    }

    // If we're not autoscrolling, and the user scrolled down,
    // we may have to re-engage the autoscroll.
    if (!autoscroll && scrollTop > oldScrollTop) {
      this.maybeEngageAutoscroll()
    }
  }

  private maybeEngageAutoscroll() {
    if (this.props.pathBuilder.isSnapshot()) {
      return
    }

    if (this.autoscrollRafId) {
      this.props.raf.cancelAnimationFrame(this.autoscrollRafId)
    }

    this.autoscrollRafId = this.props.raf.requestAnimationFrame(() => {
      let autoscroll = this.computeAutoScroll()
      if (autoscroll) {
        this.autoscroll = true
      }
    })
  }

  // Compute whether we should auto-scroll from the state of the DOM.
  // This forces a layout, so should be used sparingly.
  private computeAutoScroll(): boolean {
    let rootEl = this.rootRef.current
    if (!rootEl) {
      return true
    }

    // Always auto-scroll when we're recovering from a loading screen.
    let cursorEl = this.cursorRef.current
    if (!cursorEl) {
      return true
    }

    // Never auto-scroll if we're horizontally scrolled.
    if (rootEl.scrollLeft) {
      return false
    }

    let lastElInView =
      cursorEl.getBoundingClientRect().bottom <=
      rootEl.getBoundingClientRect().bottom
    return lastElInView
  }

  resetRender() {
    let root = this.rootRef.current
    let cursor = this.cursorRef.current
    if (root) {
      while (root.firstChild != cursor) {
        root.removeChild(root.firstChild)
      }
    }

    this.elByStoredLineIndex = {}
    this.logCheckpoint = 0
  }

  // Render new logs that have come in since the current checkpoint.
  incrementalRender() {
    let root = this.rootRef.current
    let cursor = this.cursorRef.current
    let mn = this.props.manifestName
    let logStore = this.props.logStore

    let showManifestName = !mn
    let patch = mn
      ? logStore.manifestLogPatchSet(mn, this.logCheckpoint)
      : logStore.allLogPatchSet(this.logCheckpoint)

    let lines = patch.lines.filter((line) => {
      if (this.props.hideBuildLog && line.spanId.indexOf("build:") === 0) {
        return false
      }
      if (this.props.hideRunLog && line.spanId.indexOf("build:") !== 0) {
        return false
      }
      return true
    })
    this.logCheckpoint = patch.checkpoint

    let lastManifestName = ""
    for (let i = 0; i < lines.length; i++) {
      let line = lines[i]
      let isContextChange = i > 0 && line.manifestName !== lastManifestName
      let lineEl = newLineEl(line, isContextChange, showManifestName)
      let existingLineEl = this.elByStoredLineIndex[line.storedLineIndex]
      if (existingLineEl) {
        root.replaceChild(lineEl, existingLineEl)
      } else {
        root.insertBefore(lineEl, cursor)
      }
      this.elByStoredLineIndex[line.storedLineIndex] = lineEl

      lastManifestName = line.manifestName
    }

    if (this.autoscroll) {
      this.scrollCursorIntoView()
    }
  }

  render() {
    return (
      <LogPaneRoot ref={this.rootRef}>
        <LogEnd key="logEnd" className="logEnd" ref={this.cursorRef}>
          &#9608;
        </LogEnd>
      </LogPaneRoot>
    )
  }
}

type OverviewLogPaneProps = {
  manifestName: string
  hideBuildLog?: boolean
  hideRunLog?: boolean
}

export default function OverviewLogPane(props: OverviewLogPaneProps) {
  let pathBuilder = usePathBuilder()
  let logStore = useLogStore()
  let raf = useRaf()
  return (
    <OverviewLogComponent
      manifestName={props.manifestName}
      pathBuilder={pathBuilder}
      logStore={logStore}
      raf={raf}
      hideBuildLog={props.hideBuildLog}
      hideRunLog={props.hideRunLog}
    />
  )
}
