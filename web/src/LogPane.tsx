import React, { Component } from "react"
import { ReactComponent as LogoWordmarkSvg } from "./assets/svg/logo-wordmark-gray.svg"
import AnsiLine from "./AnsiLine"
import "./LogPane.scss"
import ReactDOM from "react-dom"
import { SnapshotHighlight } from "./types"

const WHEEL_DEBOUNCE_MS = 250

type LogPaneProps = {
  log: string
  message?: string
  isExpanded: boolean
  handleSetHighlight: (highlight: SnapshotHighlight) => void
  handleClearHighlight: () => void
  highlight: SnapshotHighlight | null
  modalIsOpen: boolean
}

type LogPaneState = {
  autoscroll: boolean
  lastWheelEventTimeMs: number
}

class LogPane extends Component<LogPaneProps, LogPaneState> {
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
  }

  componentDidMount() {
    if (this.lastElement !== null) {
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
        let beginningLogLine = this.findLogLineID(beginning.parentElement)
        let endingLogLine = this.findLogLineID(end.parentElement)

        if (beginningLogLine && endingLogLine) {
          this.props.handleSetHighlight({
            beginningLogID: beginningLogLine,
            endingLogID: endingLogLine,
          })
        }
      }
    }
  }

  findLogLineID(el: HTMLElement | null): string | null {
    if (el && el.attributes.getNamedItem("data-lineid")) {
      let lineID = el.attributes.getNamedItem("data-lineid")
      if (lineID) {
        return lineID.value
      }
      return null
    } else if (el) {
      return this.findLogLineID(el.parentElement)
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
    let classes = `LogPane ${this.props.isExpanded ? "LogPane--expanded" : ""}`

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

      logLines.push(
        <div
          key={key}
          data-lineid={i}
          className={`logLine ${shouldHighlight ? "highlighted" : ""}`}
        >
          <AnsiLine line={l} />
        </div>
      )
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
