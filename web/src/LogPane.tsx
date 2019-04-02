import React, { Component, createRef } from "react"
import Ansi from "ansi-to-react"
import "./LogPane.scss"

type AnsiLineProps = {
  line: string
}
let AnsiLine = React.memo(function(props: AnsiLineProps) {
  return (
    <div>
      <Ansi linkify={false}>{props.line}</Ansi>
    </div>
  )
})

type LogPaneProps = {
  log: string
  message?: string
}
type LogPaneState = {
  autoscroll: boolean
}

class LogPane extends Component<LogPaneProps, LogPaneState> {
  private lastElement: HTMLDivElement | null = null
  private scrollTimeout: NodeJS.Timeout | null = null

  constructor(props: LogPaneProps) {
    super(props)

    this.state = {
      autoscroll: true,
    }

    this.refreshAutoScroll = this.refreshAutoScroll.bind(this)
  }

  componentDidMount() {
    if (this.lastElement !== null) {
      this.lastElement.scrollIntoView()
    }

    window.addEventListener("scroll", this.refreshAutoScroll, { passive: true })
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
    if (this.scrollTimeout) {
      clearTimeout(this.scrollTimeout)
    }
  }

  refreshAutoScroll() {
    if (this.scrollTimeout) {
      clearTimeout(this.scrollTimeout)
    }

    this.scrollTimeout = setTimeout(() => {
      let lastElInView = this.lastElement
        ? this.lastElement.getBoundingClientRect().top < window.innerHeight
        : false

      // Always auto-scroll when we're recovering from a loading screen.
      let autoscroll = false
      if (!this.props.log || !this.lastElement) {
        autoscroll = true
      } else {
        autoscroll = lastElInView
      }

      this.setState({ autoscroll })
    }, 250)
  }

  render() {
    let log = this.props.log
    if (!log || log.length == 0) {
      return <p>No logs received</p>
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

    return <div className="LogPane">{els}</div>
  }
}

export default LogPane
