import React, { Component } from "react"
import { ReactComponent as LogoWorkmarkSvg } from "./assets/svg/logo-wordmark-gray.svg"
import AnsiLine from "./AnsiLine"
import "./LogPane.scss"

const WHEEL_DEBOUNCE_MS = 250

type LogPaneProps = {
  log: string
  message?: string
  isExpanded: boolean
  podIDs: string[]
  endpoints: string[]
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
  }

  componentDidMount() {
    if (this.lastElement !== null) {
      this.lastElement.scrollIntoView()
    }

    window.addEventListener("scroll", this.refreshAutoScroll, { passive: true })
    window.addEventListener("wheel", this.handleWheel, { passive: true })
  }

  componentDidUpdate() {
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
      let lastElInView = this.lastElement
        ? this.lastElement.getBoundingClientRect().bottom < window.innerHeight
        : false

      // Always auto-scroll when we're recovering from a loading screen.
      let autoscroll = false
      if (!this.props.log || !this.lastElement) {
        autoscroll = true
      } else {
        autoscroll = lastElInView
      }

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

  render() {
    let classes = `LogPane ${this.props.isExpanded ? "LogPane--expanded" : ""}`

    let log = this.props.log
    if (!log || log.length == 0) {
      return (
        <section className={classes}>
          <section className="Pane-empty-message">
            <LogoWorkmarkSvg />
            <h2>No Logs Found</h2>
          </section>
        </section>
      )
    }

    let resourceInfo: React.ReactElement
    let endpoints = ""
    if (this.props.endpoints) {
      endpoints = this.props.endpoints.join(", ")
    }
    let podIDs = ""
    if (this.props.podIDs) {
      podIDs = this.props.podIDs.join(", ")
    }
    let endpointSuffix =
      this.props.endpoints && this.props.endpoints.length > 1 ? "s" : ""
    let podIdSuffix =
      this.props.podIDs && this.props.podIDs.length > 1 ? "s" : ""

    let resourceInfoSection = (endpoints || podIDs) && (
      <section className="resourceInfo">
        {podIDs && (
          <div>
            <span className="label">Pod ID{podIdSuffix}:</span>
            <pre>{podIDs}</pre>
          </div>
        )}
        {endpoints && (
          <div>
            <span className="label">Endpoint{endpointSuffix}:</span>
            <pre>{endpoints}</pre>
          </div>
        )}
      </section>
    )

    let logLines: Array<React.ReactElement> = []
    let lines = log.split("\n")
    logLines = lines.map(
      (line: string, i: number): React.ReactElement => {
        return <AnsiLine key={"logLine" + i} line={line} />
      }
    )
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

    return (
      <section className={classes}>
        {resourceInfoSection}
        <section className="logText">{logLines}</section>
      </section>
    )
  }
}

export default LogPane
