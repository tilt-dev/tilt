import Anser from "anser"
import React, { Component } from "react"
import styled, { keyframes } from "styled-components"
import "./LogPane.scss"
import "./LogPaneLine.scss"
import LogStore, { useLogStore } from "./LogStore"
import PathBuilder, { usePathBuilder } from "./PathBuilder"
import { SizeUnit } from "./style-helpers"
import { LogLine } from "./types"

type OverviewLogComponentProps = {
  manifestName: string
  pathBuilder: PathBuilder
  logStore: LogStore
}

let LogPaneRoot = styled.section`
  padding-top: ${SizeUnit(0.5)};
  padding-bottom: ${SizeUnit(0.5)};
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
  padding-topt: ${SizeUnit(0.25)};
  padding-left: ${SizeUnit(0.625)};
`

let anser = new Anser()

function newLineEl(line: LogLine, isContextChange: boolean): Element {
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
  let code = document.createElement("code")
  code.classList.add("LogPaneLine-content")
  code.innerHTML = anser.linkify(
    anser.ansiToHtml(anser.escapeForHtml(line.text), {
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
class OverviewLogComponent extends Component<OverviewLogComponentProps> {
  private autoscroll: boolean = false

  // The element containing all the log lines.
  private rootRef: React.RefObject<any> = React.createRef()

  // The blinking cursor at the end fo the component.
  private cursorRef: React.RefObject<HTMLParagraphElement> = React.createRef()

  // Track the scrollY of the root element to see if the user is scrolling upwards.
  private scrollY: number = -1

  // Timer for tracking autoscroll.
  private autoscrollRafID: number | null = null

  constructor(props: OverviewLogComponentProps) {
    super(props)

    this.onScroll = this.onScroll.bind(this)
  }

  scrollCursorIntoView() {
    if (this.cursorRef.current?.scrollIntoView) {
      this.cursorRef.current.scrollIntoView()
    }
  }

  componentDidUpdate(prevProps: OverviewLogComponentProps) {
    if (prevProps.logStore !== this.props.logStore) {
      // TODO(nick): setup/teardown logstore listeners.
    }

    if (prevProps.manifestName !== this.props.manifestName) {
      this.autoscroll = true
      this.scrollY = -1
      if (this.props.pathBuilder.isSnapshot()) {
        this.autoscroll = false
      }

      this.renderFreshLogs()
    } else if (prevProps.logStore !== this.props.logStore) {
      this.renderFreshLogs()
    }
  }

  componentDidMount() {
    if (this.props.pathBuilder.isSnapshot()) {
      this.autoscroll = false
    }

    window.addEventListener("scroll", this.onScroll, {
      passive: true,
    })
    this.renderFreshLogs()

    // TODO(nick): setup logstore listeners
  }

  componentWillUnmount() {
    window.removeEventListener("scroll", this.onScroll)
    if (this.autoscrollRafID) {
      cancelAnimationFrame(this.autoscrollRafID)
    }
    // TODO(nick): teardown logstore listeners
  }

  onScroll() {
    let rootEl = this.rootRef.current
    if (!rootEl) {
      return
    }

    let scrollY = rootEl.scrollY
    let oldScrollY = this.scrollY
    let autoscroll = this.autoscroll

    this.scrollY = scrollY
    if (oldScrollY === -1 || oldScrollY === scrollY) {
      return
    }

    // If we're scrolled horizontally, cancel the autoscroll.
    if (rootEl.scrollX > 0) {
      this.autoscroll = false
      return
    }

    // If we're autoscrolling, and the user scrolled up,
    // cancel the autoscroll.
    if (autoscroll && scrollY < oldScrollY) {
      this.autoscroll = false
      return
    }

    // If we're not autoscrolling, and the user scrolled down,
    // we may have to re-engage the autoscroll.
    if (!autoscroll && scrollY > oldScrollY) {
      this.maybeEngageAutoscroll()
    }
  }

  private maybeEngageAutoscroll() {
    if (this.props.pathBuilder.isSnapshot()) {
      return
    }

    if (this.autoscrollRafID) {
      cancelAnimationFrame(this.autoscrollRafID)
    }

    this.autoscrollRafID = requestAnimationFrame(() => {
      let autoscroll = this.computeAutoScroll()
      if (autoscroll) {
        this.autoscroll = true
      }
    })
  }

  // Compute whether we should auto-scroll from the state of the DOM.
  // This forces a layout, so should be used sparingly.
  private computeAutoScroll() {
    let rootEl = this.rootRef.current
    if (!rootEl) {
      return
    }

    // Always auto-scroll when we're recovering from a loading screen.
    if (!this.cursorRef.current) {
      return true
    }

    // Never auto-scroll if we're horizontally scrolled.
    if (rootEl.scrollX) {
      return false
    }

    let lastElInView =
      this.cursorRef.current.getBoundingClientRect().bottom < window.innerHeight
    return lastElInView
  }

  // Re-render all the logs from scratch.
  renderFreshLogs() {
    let root = this.rootRef.current
    let cursor = this.cursorRef.current
    let mn = this.props.manifestName
    let logStore = this.props.logStore

    let lines = mn ? logStore.manifestLog(mn) : logStore.allLog()

    while (root.firstChild != cursor) {
      root.removeChild(root.firstChild)
    }

    let lastManifestName = ""
    for (let i = 0; i < lines.length; i++) {
      let line = lines[i]
      let isContextChange = i > 0 && line.manifestName !== lastManifestName
      let lineEl = newLineEl(line, false)
      root.insertBefore(lineEl, cursor)

      lastManifestName = line.manifestName
    }

    this.scrollCursorIntoView()
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
}

export default function OverviewLogPane(props: { manifestName: string }) {
  let pathBuilder = usePathBuilder()
  let logStore = useLogStore()
  return (
    <OverviewLogComponent
      manifestName={props.manifestName}
      pathBuilder={pathBuilder}
      logStore={logStore}
    />
  )
}
