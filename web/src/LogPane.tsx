import React, { Component } from "react"
import { ReactComponent as LogoWordmarkSvg } from "./assets/svg/logo-wordmark-gray.svg"
import AnsiLine from "./AnsiLine"
import "./LogPane.scss"
import ReactDOM from "react-dom"
import { SnapshotHighlight } from "./types"

const WHEEL_DEBOUNCE_MS = 250
const LINE_ID_ATTR_NAME = "data-lineid"

type LogPaneProps = {
  log: string
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

class LogPane extends Component<LogPaneProps, LogPaneState> {
  highlightRef: React.RefObject<HTMLDivElement>
  private lastElement: HTMLDivElement | null = null
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
    this.highlightRef = React.createRef()
  }

  componentDidMount() {
    if (
      this.props.highlight &&
      this.props.isSnapshot &&
      this.highlightRef.current
    ) {
      this.highlightRef.current.scrollIntoView()
    } else if (this.lastElement !== null) {
      this.lastElement.scrollIntoView()
    }
    window.addEventListener("scroll", this.refreshAutoScroll, { passive: true })
    window.addEventListener("wheel", this.handleWheel, { passive: true })
    document.addEventListener("selectionchange", this.handleSelectionChange, {
      passive: true,
    })
  }

  componentDidUpdate(prevProps: LogPaneProps) {
    if (!this.state.autoscroll) {
      return
    }
    if (this.lastElement) {
      this.lastElement.scrollIntoView()
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

  private handleSelectionChange() {
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
        let beginningLogLine = this.findLogLineID(beginning)
        let endingLogLine = this.findLogLineID(end)

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

  findLogLineID(el: HTMLElement | Node | null): string | null {
    if (el === null) {
      return null
    }

    if (el instanceof HTMLElement && el.getAttribute(LINE_ID_ATTR_NAME)) {
      return el.getAttribute(LINE_ID_ATTR_NAME)
    } else if (el instanceof HTMLElement) {
      return this.findLogLineID(el.parentElement)
    } else if (el instanceof Node) {
      return this.findLogLineID(el.parentNode)
    }

    return null
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
    if (!this.props.log || !this.lastElement) {
      return true
    }

    // Never auto-scroll if we're horizontally scrolled.
    if (window.scrollX) {
      return false
    }

    let lastElInView =
      this.lastElement.getBoundingClientRect().bottom < window.innerHeight
    return lastElInView
  }

  render() {
    let classes = `LogPane`

    let log = this.props.log
    if (!log || log.length === 0) {
      return (
        <section className={classes}>
          <section className="Pane-empty-message">
            <LogoWordmarkSvg />
            <h2>No Logs Found</h2>
          </section>
        </section>
      )
    }

    let logLines: Array<React.ReactElement> = []
    let lines = log.split("\n")

    let sawBeginning = false
    let sawEnd = false
    let highlight = this.props.highlight
    for (let i = 0; i < lines.length; i++) {
      const l = lines[i]
      const key = "logLine" + i

      let shouldHighlight = false
      if (highlight) {
        if (highlight.beginningLogID === i.toString()) {
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
        <span
          ref={i === 0 ? this.highlightRef : null}
          key={key}
          data-lineid={i}
          className={`logLine ${shouldHighlight ? "highlighted" : ""}`}
        >
          <AnsiLine line={l} />
        </span>
      )
      logLines.push(el)
    }

    logLines.push(
      <p
        key="logEnd"
        className="logEnd"
        ref={el => {
          this.lastElement = el
        }}
      >
        &#9608;
      </p>
    )

    return <section className={classes}>{logLines}</section>
  }
}

export default LogPane
