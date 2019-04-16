import React, { Component, createRef } from "react"
import Ansi from "ansi-to-react"
import "./LogPane.scss"

type AnsiLineProps = {
  line: string
}
let AnsiLine = React.memo(function(props: AnsiLineProps) {
  return (
    <div>
      <Ansi linkify={false} useClasses={true}>
        {props.line}
      </Ansi>
    </div>
  )
})

type LogPaneProps = {
  log: string
  message?: string
  isExpanded: boolean
}
type LogPaneState = {
  autoscroll: boolean
}

class LogPane extends Component<LogPaneProps, LogPaneState> {
  private lastElement: HTMLDivElement | null = null
  private rafID: number | null = null

  constructor(props: LogPaneProps) {
    super(props)

    this.state = {
      autoscroll: true,
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
      this.setState({ autoscroll: false })
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

      this.setState({ autoscroll })
    })
  }

  render() {
    let classes = `LogPane ${this.props.isExpanded ? "LogPane--expanded" : ""}`

    let log = this.props.log
    if (!log || log.length == 0) {
      return (
        <section className={classes}>
          <p className="LogPane-empty">No logs received</p>
        </section>
      )
    }

    let els: Array<React.ReactElement> = []
    let lines = log.split("\n")
    els = lines.map(
      (line: string, i: number): React.ReactElement => {
        return <AnsiLine key={"logLine" + i} line={line} />
      }
    )
    els.push(
      <div
        key="logEnd"
        className="logEnd"
        ref={el => {
          this.lastElement = el
        }}
      >
        &#9608;
      </div>
    )

    return <section className={classes}>{els}</section>
  }
}

export default LogPane
