import React, { Component, PureComponent } from "react"
import { ReactComponent as LogoWordmarkSvg } from "./assets/svg/logo-wordmark-gray.svg"
import AnsiLine from "./AnsiLine"
import "./LogPane.scss"
import ReactDOM from "react-dom"
import { LogLine, SnapshotHighlight } from "./types"
import color from "./color"
import { SizeUnit, Width } from "./constants"
import findLogLineID from "./findLogLine"
import styled from "styled-components"

const WHEEL_DEBOUNCE_MS = 250

type LogPaneProps = {
  logLines: LogLine[]
  showManifestPrefix: boolean
  message?: string
  handleSetHighlight: (highlight: SnapshotHighlight) => void
  handleClearHighlight: () => void
  highlight: SnapshotHighlight | null | undefined
  modalIsOpen: boolean
  isSnapshot: boolean
}

type LogPaneState = {
  autoscroll: boolean
  lastWheelEventTimeMs: number
}

type LogLineComponentProps = {
  text: string
  manifestName: string
  level: string
  lineId: number
  shouldHighlight: boolean
  showManifestPrefix: boolean
  isContextChange: boolean
}

let LogLinePrefixRoot = styled.span`
  user-select: none;
  width: calc(
    ${Width.tabNav}px - ${SizeUnit(0.5)}
  ); // Match height of tab above
  box-sizing: border-box;
  display: inline-block;
  background-color: ${color.grayDark};
  border-right: 1px solid ${color.grayLightest};
  color: ${color.grayLightest};
  padding-right: ${SizeUnit(0.5)};
  margin-right: ${SizeUnit(0.5)};
  text-align: right;
  text-overflow: ellipsis;
  overflow: hidden;
  white-space: nowrap;
  flex-shrink: 0;

  &::selection {
    background-color: transparent;
  }

  .logLine.is-contextChange > & {
    margin-top: -1px;
    border-top: 1px dotted ${color.grayLightest};
  }
`

let LogLinePrefix = React.memo((props: { name: string }) => {
  let name = props.name
  if (!name) {
    name = "(global)"
  }
  return <LogLinePrefixRoot title={name}>{name}</LogLinePrefixRoot>
})

class LogLineComponent extends PureComponent<LogLineComponentProps> {
  private ref: React.RefObject<HTMLSpanElement> = React.createRef()

  scrollIntoView() {
    if (this.ref.current) {
      this.ref.current.scrollIntoView()
    }
  }

  render() {
    let props = this.props
    let prefix = null
    let text = props.text
    if (props.showManifestPrefix) {
      prefix = <LogLinePrefix name={props.manifestName} />
    }
    let classes = ["logLine"]
    if (props.shouldHighlight) {
      classes.push("highlighted")
    }
    if (props.level == "WARN") {
      classes.push("is-warning")
    }
    if (props.isContextChange) {
      classes.push("is-contextChange")
    }
    return (
      <span
        ref={this.ref}
        data-lineid={props.lineId}
        className={classes.join(" ")}
      >
        {prefix}
        <AnsiLine line={text} className={"logLine-content"} />
      </span>
    )
  }
}

class LogPane extends Component<LogPaneProps, LogPaneState> {
  highlightRef: React.RefObject<LogLineComponent> = React.createRef()
  private lastElement: React.RefObject<HTMLParagraphElement> = React.createRef()
  private rafID: number | null = null

  constructor(props: LogPaneProps) {
    super(props)

    this.state = {
      autoscroll: true,
      lastWheelEventTimeMs: 0,
    }

    this.refreshAutoScroll = this.refreshAutoScroll.bind(this)
    this.handleWheel = this.handleWheel.bind(this)
    this.handleSelectionChange = this.handleSelectionChange.bind(this)
  }

  componentDidMount() {
    if (
      this.props.highlight &&
      this.props.isSnapshot &&
      this.highlightRef.current
    ) {
      this.highlightRef.current.scrollIntoView()
    } else if (this.lastElement.current?.scrollIntoView) {
      this.lastElement.current.scrollIntoView()
    }

    if (!this.props.isSnapshot) {
      window.addEventListener("scroll", this.refreshAutoScroll, {
        passive: true,
      })
      document.addEventListener("selectionchange", this.handleSelectionChange, {
        passive: true,
      })
      window.addEventListener("wheel", this.handleWheel, { passive: true })
    }
  }

  componentDidUpdate(prevProps: LogPaneProps) {
    if (!this.state.autoscroll) {
      return
    }
    if (this.lastElement.current?.scrollIntoView) {
      this.lastElement.current.scrollIntoView()
    }
  }

  componentWillUnmount() {
    window.removeEventListener("scroll", this.refreshAutoScroll)
    window.removeEventListener("wheel", this.handleWheel)
    if (this.rafID) {
      clearTimeout(this.rafID)
    }
    document.removeEventListener("selectionchange", this.handleSelectionChange)
  }

  handleSelectionChange() {
    let selection = document.getSelection()
    if (
      selection &&
      selection.focusNode &&
      selection.anchorNode &&
      !this.props.modalIsOpen
    ) {
      let node = ReactDOM.findDOMNode(this)
      let beginning = selection.focusNode
      let end = selection.anchorNode
      let text = selection.toString()

      // if end is before beginning
      if (
        beginning.compareDocumentPosition(end) &
        Node.DOCUMENT_POSITION_PRECEDING
      ) {
        // swap beginning and end
        ;[beginning, end] = [end, beginning]
      }

      if (selection.isCollapsed) {
        this.props.handleClearHighlight()
      } else if (
        node &&
        node.contains(beginning) &&
        node.contains(end) &&
        !node.isEqualNode(beginning) &&
        !node.isEqualNode(end)
      ) {
        let beginningLogLine = findLogLineID(beginning)
        let endingLogLine = findLogLineID(end)

        if (beginningLogLine && endingLogLine) {
          this.props.handleSetHighlight({
            beginningLogID: beginningLogLine,
            endingLogID: endingLogLine,
            text: text,
          })
        }
      }
    }
  }
  private handleWheel(event: WheelEvent) {
    if (event.deltaY < 0) {
      this.setState({ autoscroll: false, lastWheelEventTimeMs: Date.now() })
    }
  }

  private refreshAutoScroll() {
    if (this.rafID) {
      cancelAnimationFrame(this.rafID)
    }

    this.rafID = requestAnimationFrame(() => {
      let autoscroll = this.computeAutoScroll()
      this.setState(prevState => {
        let lastWheelEventTimeMs = prevState.lastWheelEventTimeMs
        if (lastWheelEventTimeMs) {
          if (Date.now() - lastWheelEventTimeMs < WHEEL_DEBOUNCE_MS) {
            return prevState
          }
          return { autoscroll: false, lastWheelEventTimeMs: 0 }
        }

        return { autoscroll, lastWheelEventTimeMs: 0 }
      })
    })
  }

  // Compute whether we should auto-scroll from the state of the DOM.
  // This forces a layout, so should be used sparingly.
  computeAutoScroll() {
    // Always auto-scroll when we're recovering from a loading screen.
    if (!this.props.logLines?.length || !this.lastElement.current) {
      return true
    }

    // Never auto-scroll if we're horizontally scrolled.
    if (window.scrollX) {
      return false
    }

    let lastElInView =
      this.lastElement.current.getBoundingClientRect().bottom <
      window.innerHeight
    return lastElInView
  }

  render() {
    let classes = `LogPane`

    let lines = this.props.logLines
    if (!lines || lines.length === 0) {
      return (
        <section className={classes}>
          <section className="Pane-empty-message">
            <LogoWordmarkSvg />
            <h2>No Logs Found</h2>
          </section>
        </section>
      )
    }

    let logLineEls: Array<React.ReactElement> = []

    let sawBeginning = false
    let sawEnd = false
    let highlight = this.props.highlight
    let lastManifestName = ""
    for (let i = 0; i < lines.length; i++) {
      const l = lines[i]
      const key = "logLine" + i

      let shouldHighlight = false
      let maybeHighlightRef = null
      if (highlight) {
        if (highlight.beginningLogID === i.toString()) {
          maybeHighlightRef = this.highlightRef
          sawBeginning = true
        }
        if (highlight.endingLogID === i.toString()) {
          shouldHighlight = true
          sawEnd = true
        }
        if (sawBeginning && !sawEnd) {
          shouldHighlight = true
        }
      }

      let isContextChange = i > 0 && l.manifestName != lastManifestName
      let el = (
        <LogLineComponent
          ref={maybeHighlightRef}
          key={key}
          text={l.text}
          level={l.level}
          manifestName={l.manifestName}
          isContextChange={isContextChange}
          lineId={i}
          showManifestPrefix={this.props.showManifestPrefix}
          shouldHighlight={shouldHighlight}
        />
      )
      logLineEls.push(el)

      lastManifestName = l.manifestName
    }

    logLineEls.push(
      <p key="logEnd" className="logEnd" ref={this.lastElement}>
        &#9608;
      </p>
    )

    return <section className={classes}>{logLineEls}</section>
  }
}

export default LogPane
