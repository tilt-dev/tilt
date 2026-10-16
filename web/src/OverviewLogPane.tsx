import React, { Component } from "react"
import { useNavigate, useLocation } from "react-router-dom"
import styled, { keyframes } from "styled-components"
import { FilterSet, filterSetsEqual } from "./logfilters"
import "./LogLine.scss"
import "./LogPane.scss"
import LogStore, {
  LogUpdateAction,
  LogUpdateEvent,
  useLogStore,
} from "./LogStore"
import {
  findKey,
  formatValueCompact,
  levelKeys,
  messageKeys,
  parseStructured,
  ParsedLogLine,
  StructuredKind,
} from "./logparse"
import { DISPLAY_LOG_PROLOGUE_LENGTH, formatLogTime, LogDisplay } from "./logs"
import { useSelectedLogLine } from "./logselection"
import PathBuilder, { usePathBuilder } from "./PathBuilder"
import { RafContext, useRaf } from "./raf"
import { isTargetEditable } from "./shortcut"
import { useStarredResources } from "./StarredResourcesContext"
import { Color, FontSize, SizeUnit } from "./style-helpers"
import Anser from "./third-party/anser/index.js"
import { LogLine, ResourceName } from "./types"

// The number of lines to display before an error.
export const PROLOGUE_LENGTH = DISPLAY_LOG_PROLOGUE_LENGTH

type OverviewLogComponentProps = {
  manifestName: string
  pathBuilder: PathBuilder
  logStore: LogStore
  raf: RafContext
  filterSet: FilterSet
  navigate: ReturnType<typeof useNavigate>
  scrollToStoredLineIndex: number | null
  starredResources: string[]

  // The storedLineIndex of the line shown in the details drawer, if any.
  selectedLineIndex: number | null
  onSelectLine: (index: number | null) => void
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

  if (buildEvent !== "init") {
    let time = document.createElement("span")
    time.className = "LogLine-time"
    time.textContent = formatLogTime(line.time)
    if (line.time) {
      time.title = line.time
    }
    span.appendChild(time)
  }

  let parsed: ParsedLogLine = buildEvent
    ? { kind: StructuredKind.Text }
    : parseStructured(text)
  if (parsed.kind !== StructuredKind.Text) {
    span.appendChild(newStructuredContentEl(parsed))
    return span
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

function appendTextEl(parent: Element, className: string, text: string) {
  let el = document.createElement("span")
  el.className = className
  el.textContent = text
  parent.appendChild(el)
  return el
}

// Renders a JSON/logfmt line as a compact summary:
//   [prefix] LEVEL message key=value key=value
function newStructuredContentEl(
  parsed: Exclude<ParsedLogLine, { kind: StructuredKind.Text }>
): Element {
  let code = document.createElement("code")
  code.classList.add("LogLine-content", "is-structured")

  let data = parsed.data
  if (parsed.prefix) {
    appendTextEl(code, "LogLine-structPrefix", parsed.prefix)
  }

  let levelKey = findKey(data, levelKeys)
  if (levelKey) {
    let level = String(data[levelKey])
    let el = appendTextEl(code, "LogLine-structLevel", level.toUpperCase())
    el.setAttribute("data-level", level.toLowerCase())
  }

  let msgKey = findKey(data, messageKeys)
  if (msgKey) {
    appendTextEl(code, "LogLine-structMsg", String(data[msgKey]))
  }

  for (let key of Object.keys(data)) {
    if (key === msgKey || key === levelKey) {
      continue
    }
    let field = document.createElement("span")
    field.className = "LogLine-field"
    appendTextEl(field, "LogLine-key", key)
    let value = data[key]
    let valueEl = appendTextEl(field, "LogLine-val", formatValueCompact(value))
    if (typeof value === "string") {
      valueEl.classList.add("is-string")
    }
    code.appendChild(field)
  }

  // newline ensures this takes up at least one line
  code.appendChild(document.createTextNode("\n"))
  return code
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
  scrollTarget: number | null | undefined = null

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

  private logDisplay: LogDisplay

  constructor(props: OverviewLogComponentProps) {
    super(props)

    this.logDisplay = new LogDisplay(props.filterSet)
    this.onScroll = this.onScroll.bind(this)
    this.onLogUpdate = this.onLogUpdate.bind(this)
    this.renderBuffer = this.renderBuffer.bind(this)
    this.onClick = this.onClick.bind(this)
    this.onKeydown = this.onKeydown.bind(this)
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

      // Truncation re-numbers the stored lines, so the selection
      // no longer points at the same line.
      if (this.props.selectedLineIndex !== null) {
        this.props.onSelectLine(null)
      }
    }

    this.readLogsFromLogStore()
  }

  componentDidUpdate(prevProps: OverviewLogComponentProps) {
    if (prevProps.selectedLineIndex !== this.props.selectedLineIndex) {
      this.updateSelectedLineEl()
    }

    if (prevProps.logStore !== this.props.logStore) {
      prevProps.logStore.removeUpdateListener(this.onLogUpdate)
      this.props.logStore.addUpdateListener(this.onLogUpdate)
    }

    if (
      prevProps.manifestName !== this.props.manifestName ||
      !filterSetsEqual(prevProps.filterSet, this.props.filterSet)
    ) {
      this.resetRender()

      this.scrollTarget = this.props.scrollToStoredLineIndex
      if (typeof this.scrollTarget === "number") {
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

    // On first load, scroll to either the line we navigated to,
    // or the line selected in the URL (i.e., a permalink).
    this.scrollTarget =
      this.props.scrollToStoredLineIndex ?? this.props.selectedLineIndex
    if (typeof this.scrollTarget == "number") {
      this.needsScrollToLine = true
    }
    this.autoscroll = !this.needsScrollToLine

    rootEl.addEventListener("scroll", this.onScroll, {
      passive: true,
    })
    rootEl.addEventListener("click", this.onClick)
    document.body.addEventListener("keydown", this.onKeydown)
    this.resetRender()
    this.readLogsFromLogStore()

    this.props.logStore.addUpdateListener(this.onLogUpdate)
  }

  componentWillUnmount() {
    this.props.logStore.removeUpdateListener(this.onLogUpdate)
    document.body.removeEventListener("keydown", this.onKeydown)

    let rootEl = this.rootRef.current
    if (!rootEl) {
      return
    }
    rootEl.removeEventListener("scroll", this.onScroll)
    rootEl.removeEventListener("click", this.onClick)

    if (this.autoscrollRafId) {
      this.props.raf.cancelAnimationFrame(this.autoscrollRafId)
    }
  }

  // Clicking a line selects it (opening the details drawer).
  onClick(e: MouseEvent) {
    let target = e.target as Element | null
    if (!target || !target.closest) {
      return
    }

    // Let links and buttons (e.g., the alert nav) do their own thing.
    if (target.closest("a, button")) {
      return
    }

    // Don't steal clicks from users selecting text to copy.
    let selection = window.getSelection()
    if (selection && !selection.isCollapsed && selection.toString()) {
      return
    }

    let lineEl = target.closest(".LogLine[data-sl-index]")
    if (!lineEl || lineEl.classList.contains("is-buildEvent")) {
      return
    }

    let index = Number(lineEl.getAttribute("data-sl-index"))
    if (index === this.props.selectedLineIndex) {
      this.props.onSelectLine(null)
      return
    }

    // The user is inspecting a line, so stop following new logs.
    this.autoscroll = false
    this.props.onSelectLine(index)
  }

  // While a line is selected, up/down move the selection and escape clears it.
  onKeydown(e: KeyboardEvent) {
    let selected = this.props.selectedLineIndex
    if (selected === null || isTargetEditable(e)) {
      return
    }
    if (e.shiftKey || e.altKey || e.metaKey || e.ctrlKey || e.isComposing) {
      return
    }

    switch (e.key) {
      case "Escape":
        this.props.onSelectLine(null)
        e.preventDefault()
        return

      case "ArrowUp":
      case "ArrowDown":
        let el = this.lineHashList.lookupByStoredLineIndex(selected)?.el
        if (!el) {
          return
        }
        e.preventDefault()

        let next = this.adjacentSelectableLineEl(el, e.key === "ArrowDown")
        if (!next) {
          return
        }

        this.autoscroll = false
        next.scrollIntoView?.({ block: "nearest" })
        this.props.onSelectLine(Number(next.getAttribute("data-sl-index")))
        return
    }
  }

  private adjacentSelectableLineEl(el: Element, forward: boolean) {
    let cur: Element | null = el
    while (true) {
      cur = forward ? cur.nextElementSibling : cur.previousElementSibling
      if (!cur) {
        return null
      }
      if (
        cur.hasAttribute("data-sl-index") &&
        !cur.classList.contains("is-buildEvent")
      ) {
        return cur
      }
    }
  }

  // The element currently marked as selected.
  private selectedLineEl: Element | null = null

  private updateSelectedLineEl() {
    this.selectedLineEl?.classList.remove("is-highlighted")
    this.selectedLineEl = null

    let selected = this.props.selectedLineIndex
    if (selected === null) {
      return
    }
    let el = this.lineHashList.lookupByStoredLineIndex(selected)?.el
    if (el) {
      el.classList.add("is-highlighted")
      this.selectedLineEl = el
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
    this.selectedLineEl = null
    this.logDisplay = new LogDisplay(this.props.filterSet)
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

    let lines = this.logDisplay.filterLines(patch.lines)

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
        this.scrollTarget as number
      )
      if (!entry) {
        // The line isn't in this view (e.g., it's been filtered out),
        // so it will never render.
        this.needsScrollToLine = false
      } else if (entry.el) {
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

    let shouldDisplayPrologues = this.logDisplay.shouldDisplayPrologues()
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
      this.logDisplay.matchesLevelFilter(line) &&
      (!entry.next || entry.next?.line.level !== line.level)
    if (isEndOfAlert) {
      extraClasses.push("is-endOfAlert")
    }

    let isStartOfAlert =
      shouldDisplayPrologues &&
      !line.buildEvent &&
      !this.logDisplay.matchesLevelFilter(line) &&
      (!entry.prev ||
        this.logDisplay.matchesLevelFilter(entry.prev.line) ||
        entry.prev.line.buildEvent)
    if (isStartOfAlert) {
      extraClasses.push("is-startOfAlert")
    }

    if (line.storedLineIndex === this.props.selectedLineIndex) {
      extraClasses.push("is-highlighted")
    }

    let lineEl = newLineEl(entry.line, showManifestName, extraClasses)
    if (line.storedLineIndex === this.props.selectedLineIndex) {
      this.selectedLineEl = lineEl
    }
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
  // Test-only: capture the underlying class component instance. React 18
  // dropped the ability to read an instance back from a render() call, so
  // tests that need to poke at OverviewLogComponent directly pass a ref here.
  innerRef?: React.Ref<OverviewLogComponent>
}

export default function OverviewLogPane(props: OverviewLogPaneProps) {
  const navigate = useNavigate()
  let location = useLocation() as any
  let pathBuilder = usePathBuilder()
  let logStore = useLogStore()
  let raf = useRaf()
  let starredContext = useStarredResources()
  let [selectedLineIndex, setSelectedLineIndex] = useSelectedLogLine()

  return (
    <OverviewLogComponent
      ref={props.innerRef}
      manifestName={props.manifestName}
      pathBuilder={pathBuilder}
      logStore={logStore}
      raf={raf}
      filterSet={props.filterSet}
      navigate={navigate}
      scrollToStoredLineIndex={location?.state?.storedLineIndex}
      starredResources={starredContext.starredResources}
      selectedLineIndex={selectedLineIndex}
      onSelectLine={setSelectedLineIndex}
    />
  )
}
