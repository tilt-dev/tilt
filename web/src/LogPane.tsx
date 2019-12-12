import React, { Component, PureComponent } from "react"
import { ReactComponent as LogoWordmarkSvg } from "./assets/svg/logo-wordmark-gray.svg"
import AnsiLine from "./AnsiLine"
import "./LogPane.scss"
import ReactDOM from "react-dom"
import { LogLine, SnapshotHighlight } from "./types"
import { sourcePrefix } from "./logs"
import findLogLineID from "./findLogLine"

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
  lineId: number
  shouldHighlight: boolean
  showManifestPrefix: boolean
}

class LogLineComponent extends PureComponent<LogLineComponentProps> {
  private ref: React.RefObject<HTMLSpanElement> = React.createRef()

  scrollIntoView() {
    if (this.ref.current) {
      this.ref.current.scrollIntoView()
    }
  }

  render() {
    let props = this.props
    let text = props.text
    if (props.showManifestPrefix) {
      text = sourcePrefix(props.manifestName) + text
    }
    return (
      <span
        ref={this.ref}
        data-lineid={props.lineId}
        className={`logLine ${props.shouldHighlight ? "highlighted" : ""}`}
      >
        <AnsiLine line={text} />
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

      let el = (
        <LogLineComponent
          ref={maybeHighlightRef}
          key={key}
          text={l.text}
          manifestName={l.manifestName}
          lineId={i}
          showManifestPrefix={this.props.showManifestPrefix}
          shouldHighlight={shouldHighlight}
        />
      )
      logLineEls.push(el)
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
