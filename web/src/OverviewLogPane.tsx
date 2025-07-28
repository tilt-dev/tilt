import React, { Component } from "react"
import { useNavigate, useLocation } from "react-router-dom"
import styled, { keyframes } from "styled-components"
import {
  FilterLevel,
  FilterSet,
  filterSetsEqual,
  FilterSource,
  TermState,
} from "./logfilters"
import "./LogLine.scss"
import "./LogPane.scss"
import LogStore, {
  LogUpdateAction,
  LogUpdateEvent,
  useLogStore,
} from "./LogStore"
import { isBuildSpanId } from "./logs"
import PathBuilder, { usePathBuilder } from "./PathBuilder"
import { RafContext, useRaf } from "./raf"
import { useStarredResources } from "./StarredResourcesContext"
import { Color, FontSize, SizeUnit } from "./style-helpers"
import Anser from "./third-party/anser/index.js"
import { LogLevel, LogLine, ResourceName } from "./types"

// The number of lines to display before an error.
export const PROLOGUE_LENGTH = 5

type OverviewLogComponentProps = {
  manifestName: string
  pathBuilder: PathBuilder
  logStore: LogStore
  raf: RafContext
  filterSet: FilterSet
  navigate: ReturnType<typeof useNavigate>
  scrollToStoredLineIndex: number | null
  starredResources: string[]
}

let LogPaneRoot = styled.section`
  padding: 0 0 ${SizeUnit(0.25)} 0;
  background-color: ${Color.gray10};
  width: 100%;
  height: 100%;
  overflow-y: auto;
  box-sizing: border-box;
  font-size: ${FontSize.smallest};
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
  font-size: var(--log-font-scale);
`

let anser = new Anser()

function newLineEl(
  line: LogLine,
  showManifestPrefix: boolean,
  extraClasses: string[]
): Element {
  let text = line.text
  let level = line.level
  let buildEvent = line.buildEvent
  let classes = ["LogLine"]
  classes.push(...extraClasses)
  if (level === "WARN") {
    classes.push("is-warning")
  } else if (level === "ERROR") {
    classes.push("is-error")
  }
  if (buildEvent === "init") {
    classes.push("is-buildEvent")
    classes.push("is-buildEvent-init")

    if (showManifestPrefix) {
      // For build event lines, we put the manifest name is a suffix
      // rather than a prefix, because it looks nicer.
      text += ` • ${line.manifestName}`
    } else {
      // If we're viewing a single resource, we should make the build event log
      // lines sticky, so that we always know context of the current logs.
      classes.push("is-sticky")
    }
  }
  if (buildEvent === "fallback") {
    classes.push("is-buildEvent")
    classes.push("is-buildEvent-fallback")
  }
  let span = document.createElement("span")
  span.setAttribute("data-sl-index", String(line.storedLineIndex))
  span.classList.add(...classes)

  if (showManifestPrefix && buildEvent !== "init") {
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
  code.classList.add("LogLine-content")

  // newline ensures this takes up at least one line
  let spacer = "\n"
  code.innerHTML = anser.linkify(
    anser.ansiToHtml(anser.escapeForHtml(text) + spacer, {
      // Let anser colorize the html as it appears from various consoles
      use_classes: false,
    })
  )
  span.appendChild(code)
  return span
}

// An index of lines such that lets us find:
// - The next line
// - The previous line
// - The line by stored line index.
type LineHashListEntry = {
  prev?: LineHashListEntry | null
  next?: LineHashListEntry | null
  line: LogLine
  el?: Element
}

class LineHashList {
  private last: LineHashListEntry | null = null
  private byStoredLineIndex: { [key: number]: LineHashListEntry } = {}

  lookup(line: LogLine): LineHashListEntry | null {
    return this.byStoredLineIndex[line.storedLineIndex]
  }

  lookupByStoredLineIndex(storedLineIndex: number): LineHashListEntry | null {
    return this.byStoredLineIndex[storedLineIndex]
  }

  append(line: LogLine) {
    let existing = this.byStoredLineIndex[line.storedLineIndex]
    if (existing) {
      existing.line = line
    } else {
      let last = this.last
      let newEntry = { prev: last, line: line }
      this.byStoredLineIndex[line.storedLineIndex] = newEntry
      if (last) {
        last.next = newEntry
      }
      this.last = newEntry
    }
  }
}

// The number of lines to render at a time.
export const renderWindow = 250

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
  needsScrollToLine: boolean = false

  // The element containing all the log lines.
  rootRef: React.RefObject<any> = React.createRef()

  // The blinking cursor at the end of the component.
  private cursorRef: React.RefObject<HTMLParagraphElement> = React.createRef()

  // Track the scrollTop of the root element to see if the user is scrolling upwards.
  scrollTop: number = -1

  // Timer for tracking autoscroll.
  autoscrollRafId: number | null = null

  // Timer for tracking render
  renderBufferRafId: number | null = null

  // Lines to render at the end of the pane.
  forwardBuffer: LogLine[] = []

  // Lines to render at the start of the pane.
  backwardBuffer: LogLine[] = []

  private logCheckpoint: number = 0

  private lineHashList: LineHashList = new LineHashList()

  // When we're displaying warnings or errors, we want to display the last
  // N lines before the error. So we keep track of the last N lines for each span.
  private prologuesBySpanId: { [key: string]: LogLine[] } = {}

  constructor(props: OverviewLogComponentProps) {
    super(props)

    this.onScroll = this.onScroll.bind(this)
    this.onLogUpdate = this.onLogUpdate.bind(this)
    this.renderBuffer = this.renderBuffer.bind(this)
  }

  scrollCursorIntoView() {
    if (this.cursorRef.current?.scrollIntoView) {
      this.cursorRef.current.scrollIntoView()
    }
  }

  onLogUpdate(e: LogUpdateEvent) {
    if (!this.rootRef.current || !this.cursorRef.current) {
      return
    }

    if (e.action === LogUpdateAction.truncate) {
      this.resetRender()
    }

    this.readLogsFromLogStore()
  }

  componentDidUpdate(prevProps: OverviewLogComponentProps) {
    if (prevProps.logStore !== this.props.logStore) {
      prevProps.logStore.removeUpdateListener(this.onLogUpdate)
      this.props.logStore.addUpdateListener(this.onLogUpdate)
    }

    if (
      prevProps.manifestName !== this.props.manifestName ||
      !filterSetsEqual(prevProps.filterSet, this.props.filterSet)
    ) {
      this.resetRender()

      if (typeof this.props.scrollToStoredLineIndex === "number") {
        this.needsScrollToLine = true
      }
      this.autoscroll = !this.needsScrollToLine

      this.readLogsFromLogStore()
    } else if (prevProps.logStore !== this.props.logStore) {
      this.resetRender()
      this.readLogsFromLogStore()
    }
  }

  componentDidMount() {
    let rootEl = this.rootRef.current
    if (!rootEl) {
      return
    }

    if (typeof this.props.scrollToStoredLineIndex == "number") {
      this.needsScrollToLine = true
    }
    this.autoscroll = !this.needsScrollToLine

    rootEl.addEventListener("scroll", this.onScroll, {
      passive: true,
    })
    this.resetRender()
    this.readLogsFromLogStore()

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
      if (this.autoscroll) {
        this.autoscroll = false
        this.maybeScheduleRender()
      }
      return
    }

    // If we're autoscrolling, and the user scrolled up,
    // cancel the autoscroll.
    if (autoscroll && scrollTop < oldScrollTop) {
      if (this.autoscroll) {
        this.autoscroll = false
        this.maybeScheduleRender()
      }
      return
    }

    // If we're not autoscrolling, and the user scrolled down,
    // we may have to re-engage the autoscroll.
    if (!autoscroll && scrollTop > oldScrollTop) {
      this.maybeEngageAutoscroll()
    }
  }

  private maybeEngageAutoscroll() {
    // We don't expect new log lines in snapshots. So when we scroll down, we don't need
    // to worry about re-engaging autoscroll.
    if (this.props.pathBuilder.isSnapshot()) {
      return
    }

    if (this.needsScrollToLine) {
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

    this.lineHashList = new LineHashList()
    this.prologuesBySpanId = {}
    this.logCheckpoint = 0
    this.scrollTop = -1

    if (this.renderBufferRafId) {
      this.props.raf.cancelAnimationFrame(this.renderBufferRafId)
      this.renderBufferRafId = 0
    }

    if (this.autoscrollRafId) {
      this.props.raf.cancelAnimationFrame(this.autoscrollRafId)
      this.autoscrollRafId = 0
    }
  }

  matchesTermFilter(line: LogLine): boolean {
    const { term } = this.props.filterSet

    // Don't consider a filter term if the term hasn't been parsed for matching
    if (!term || term.state !== TermState.Parsed) {
      return true
    }

    return term.regexp.test(line.text)
  }

  // If we have a level filter on, check if this line matches the level filter.
  matchesLevelFilter(line: LogLine): boolean {
    let level = this.props.filterSet.level
    if (level === FilterLevel.warn && line.level !== LogLevel.WARN) {
      return false
    }
    if (level === FilterLevel.error && line.level !== LogLevel.ERROR) {
      return false
    }
    return true
  }

  // Check if this line matches the current filter.
  matchesFilter(line: LogLine): boolean {
    if (line.buildEvent) {
      // Always leave in build event logs.
      // This makes it easier to see which logs belong to which builds.
      return true
    }

    let source = this.props.filterSet.source
    if (source === FilterSource.runtime && isBuildSpanId(line.spanId)) {
      return false
    }
    if (source === FilterSource.build && !isBuildSpanId(line.spanId)) {
      return false
    }

    return this.matchesLevelFilter(line) && this.matchesTermFilter(line)
  }

  // Index this line so that we can display prologues to errors.
  trackPrologueLine(line: LogLine) {
    if (!this.prologuesBySpanId[line.spanId]) {
      this.prologuesBySpanId[line.spanId] = []
    }
    this.prologuesBySpanId[line.spanId].push(line)
  }

  // Gets the prologue for the given span, and clear the lines used for prologuing.
  getAndClearPrologue(spanId: string): LogLine[] {
    let lines = this.prologuesBySpanId[spanId]
    if (!lines) {
      return []
    }

    delete this.prologuesBySpanId[spanId]
    return lines.slice(-PROLOGUE_LENGTH) // last N lines
  }

  // Render new logs that have come in since the current checkpoint.
  readLogsFromLogStore() {
    let mn = this.props.manifestName
    let logStore = this.props.logStore
    let startCheckpoint = this.logCheckpoint

    let patch = mn
      ? mn === ResourceName.starred
        ? logStore.starredLogPatchSet(
            this.props.starredResources,
            startCheckpoint
          )
        : logStore.manifestLogPatchSet(mn, startCheckpoint)
      : logStore.allLogPatchSet(startCheckpoint)

    let lines: LogLine[] = []
    let shouldDisplayPrologues = this.props.filterSet.level !== FilterLevel.all

    patch.lines.forEach((line) => {
      let matches = this.matchesFilter(line)
      if (matches) {
        if (shouldDisplayPrologues) {
          lines.push(...this.getAndClearPrologue(line.spanId))
        }
        lines.push(line)
        return
      } else if (shouldDisplayPrologues) {
        this.trackPrologueLine(line)
      }
    })

    this.logCheckpoint = patch.checkpoint
    lines.forEach((line) => this.lineHashList.append(line))

    if (startCheckpoint) {
      // If this is an incremental render, put the lines in the forward buffer.
      lines.forEach((line) => {
        this.forwardBuffer.push(line)
      })
    } else {
      // If this is the first render, put the lines in the backward buffer, so
      // that the last lines get rendered first.
      lines.forEach((line) => {
        this.backwardBuffer.push(line)
      })
    }

    this.maybeScheduleRender()
  }

  // Schedule a render job if there's not one already scheduled.
  maybeScheduleRender() {
    if (this.renderBufferRafId) return
    this.renderBufferRafId = this.props.raf.requestAnimationFrame(
      this.renderBuffer
    )
  }

  shouldRenderForwardBuffer(): boolean {
    return this.forwardBuffer.length > 0
  }

  // When we're in autoscrolling mode, rendering the backwards buffer makes the
  // screen jiggle, because we have to render a few rows, then scroll down, then
  // render a few rows, then scroll down.
  //
  // So when in autoscrol mode, only render until we have the "last window" of logs.
  shouldRenderBackwardBuffer(): boolean {
    if (this.backwardBuffer.length == 0) {
      // Skip rendering if there's no lines in the buffer.
      return false
    }

    if (!this.autoscroll) {
      // Do render if we're scrolling up.
      return true
    }

    // In autoscroll mode, only render if there aren't enough lines to fill the viewport.
    return this.rootRef.current.scrollTop == 0
  }

  // We have two render buffers:
  // - a buffer of newer logs that we haven't rendered yet.
  // - a buffer of older logs that we haven't rendered yet.
  // First, process the newer logs.
  // If we're out of new logs to render, go back through the old logs.
  //
  // Each invocation of this method renders up to 2x renderWindow logs.
  // If there are still logs left to render, it yields the thread and schedules
  // another render.
  renderBuffer() {
    this.renderBufferRafId = 0

    let root = this.rootRef.current
    let cursor = this.cursorRef.current
    if (!root || !cursor) {
      return
    }

    if (
      !this.shouldRenderForwardBuffer() &&
      !this.shouldRenderBackwardBuffer()
    ) {
      return
    }

    // Render the lines in the forward buffer first.
    let forwardLines = this.forwardBuffer.slice(0, renderWindow)
    this.forwardBuffer = this.forwardBuffer.slice(renderWindow)
    for (let i = 0; i < forwardLines.length; i++) {
      let line = forwardLines[i]
      this.renderLineHelper(line)
    }

    if (this.shouldRenderBackwardBuffer()) {
      let backwardStart = Math.max(0, this.backwardBuffer.length - renderWindow)
      let backwardLines = this.backwardBuffer.slice(backwardStart)
      this.backwardBuffer = this.backwardBuffer.slice(0, backwardStart)

      for (let i = backwardLines.length - 1; i >= 0; i--) {
        let line = backwardLines[i]
        this.renderLineHelper(line)
      }
    }

    if (this.autoscroll) {
      this.scrollCursorIntoView()
    }

    if (this.needsScrollToLine) {
      let entry = this.lineHashList.lookupByStoredLineIndex(
        this.props.scrollToStoredLineIndex as number
      )
      if (entry?.el) {
        entry.el.scrollIntoView({ block: "center" })
        this.needsScrollToLine = false
      }
    }

    if (this.shouldRenderForwardBuffer() || this.shouldRenderBackwardBuffer()) {
      this.renderBufferRafId = this.props.raf.requestAnimationFrame(
        this.renderBuffer
      )
    }
  }

  // Creates a DOM element with a permalink to an alert.
  newAlertNavEl(line: LogLine) {
    let div = document.createElement("button")
    div.className = "LogLine-alertNav"
    div.innerHTML = "… (more) …"
    div.onclick = (e) => {
      let storedLineIndex = line.storedLineIndex
      this.props.navigate(
        this.props.pathBuilder.encpath`/r/${line.manifestName}/overview`,
        { state: { storedLineIndex } }
      )
    }
    return div
  }

  // Helper function for rendering lines. Returns true if the line was
  // successfully rendered.
  //
  // If the line has already been rendered, replace the rendered line.
  //
  // If it hasn't been rendered, but the next line has, put it before the next line.
  //
  // If it hasn't been rendered, but the previous line has, put it after the previous line.
  //
  // Otherwise, iterate through the lines until we find a place to put it.
  renderLineHelper(line: LogLine) {
    let entry = this.lineHashList.lookup(line)
    if (!entry) {
      // If the entry has been removed from the hash list for some reason,
      // just ignore it.
      return
    }

    let shouldDisplayPrologues = this.props.filterSet.level !== FilterLevel.all
    let mn = this.props.manifestName
    let showManifestName = !mn || mn === ResourceName.starred
    let prevManifestName = entry.prev?.line.manifestName || ""

    let extraClasses = []
    let isContextChange = !!entry.prev && prevManifestName !== line.manifestName
    if (isContextChange) {
      extraClasses.push("is-contextChange")
    }

    let isEndOfAlert =
      shouldDisplayPrologues &&
      this.matchesLevelFilter(line) &&
      (!entry.next || entry.next?.line.level !== line.level)
    if (isEndOfAlert) {
      extraClasses.push("is-endOfAlert")
    }

    let isStartOfAlert =
      shouldDisplayPrologues &&
      !line.buildEvent &&
      !this.matchesLevelFilter(line) &&
      (!entry.prev ||
        this.matchesLevelFilter(entry.prev.line) ||
        entry.prev.line.buildEvent)
    if (isStartOfAlert) {
      extraClasses.push("is-startOfAlert")
    }

    let lineEl = newLineEl(entry.line, showManifestName, extraClasses)
    if (isStartOfAlert) {
      lineEl.appendChild(this.newAlertNavEl(entry.line))
    }

    let root = this.rootRef.current
    let existingLineEl = entry.el
    if (existingLineEl) {
      root.replaceChild(lineEl, existingLineEl)
      entry.el = lineEl
      return
    }

    let nextEl = entry.next?.el
    if (nextEl) {
      root.insertBefore(lineEl, nextEl)
      entry.el = lineEl
      return
    }

    let prevEl = entry.prev?.el
    if (prevEl) {
      root.insertBefore(lineEl, prevEl.nextSibling)
      entry.el = lineEl
      return
    }

    // In the worst case scenario, we iterate through all lines to find a suitable place.
    let cursor = this.cursorRef.current
    for (let i = 0; i < root.children.length; i++) {
      let child = root.children[i]
      if (
        child == cursor ||
        Number(child.getAttribute("data-sl-index")) > line.storedLineIndex
      ) {
        root.insertBefore(lineEl, child)
        entry.el = lineEl
        return
      }
    }
  }

  render() {
    return (
      <LogPaneRoot ref={this.rootRef} aria-label="Log pane">
        <LogEnd key="logEnd" className="logEnd" ref={this.cursorRef}>
          &#9608;
        </LogEnd>
      </LogPaneRoot>
    )
  }
}

type OverviewLogPaneProps = {
  manifestName: string
  filterSet: FilterSet
}

export default function OverviewLogPane(props: OverviewLogPaneProps) {
  const navigate = useNavigate()
  let location = useLocation() as any
  let pathBuilder = usePathBuilder()
  let logStore = useLogStore()
  let raf = useRaf()
  let starredContext = useStarredResources()

  return (
    <OverviewLogComponent
      manifestName={props.manifestName}
      pathBuilder={pathBuilder}
      logStore={logStore}
      raf={raf}
      filterSet={props.filterSet}
      navigate={navigate}
      scrollToStoredLineIndex={location?.state?.storedLineIndex}
      starredResources={starredContext.starredResources}
    />
  )
}
