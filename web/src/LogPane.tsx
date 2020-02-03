import React, { Component, PureComponent } from "react"
import { ReactComponent as LogoWordmarkSvg } from "./assets/svg/logo-wordmark-gray.svg"
import ReactDOM from "react-dom"
import { LogLine, SnapshotHighlight } from "./types"
import LogPaneLine from "./LogPaneLine"
import findLogLineID from "./findLogLine"
import styled, { keyframes } from "styled-components"
import { Color, ColorRGBA, ColorAlpha, SizeUnit } from "./style-helpers"
import selection from "./selection"
import "./LogPane.scss"

type LogPaneProps = {
  manifestName: string
  logLines: LogLine[]
  showManifestPrefix: boolean
  message?: string
  handleSetHighlight: (highlight: SnapshotHighlight) => void
  handleClearHighlight: () => void
  highlight: SnapshotHighlight | null | undefined
  isSnapshot: boolean
}

type LogPaneState = {
  // The number of log lines to display
  renderWindow: number
}

const renderWindowDefault = 50
const renderWindowMinStep = 50

// Rough estimate of the height of a log line.
// Notice that log lines may have multiple visual lines of text, so
// in practice the height is variable.
const blankLogLineHeight = 30

let LogPaneRoot = styled.section`
  margin-top: ${SizeUnit(0.5)};
  margin-bottom: ${SizeUnit(0.5)};
  width: 100%;
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
  margin-top: ${SizeUnit(0.5)};
  margin-left: ${SizeUnit(0.5)};
`

class LogPane extends Component<LogPaneProps, LogPaneState> {
  highlightRef: React.RefObject<LogPaneLine> = React.createRef()
  private lastElement: React.RefObject<HTMLParagraphElement> = React.createRef()
  private autoscrollRafID: number | null = null
  private renderWindowRafID: number | null = null

  // Whether we're auto-scrolling to the bottom of the screen.
  private autoscroll: boolean

  // Track the pageYOffset to see if the user is scrolling upwards.
  private pageYOffset: number

  constructor(props: LogPaneProps) {
    super(props)

    this.autoscroll = true
    this.pageYOffset = -1
    this.state = {
      renderWindow: renderWindowDefault,
    }

    this.onScroll = this.onScroll.bind(this)
    this.handleSelectionChange = this.handleSelectionChange.bind(this)
  }

  componentDidMount() {
    if (this.props.isSnapshot) {
      this.autoscroll = false
    }

    if (
      this.props.highlight &&
      this.props.isSnapshot &&
      this.highlightRef.current
    ) {
      this.highlightRef.current.scrollIntoView()
    } else if (this.lastElement.current?.scrollIntoView) {
      this.lastElement.current.scrollIntoView()
    }

    window.addEventListener("scroll", this.onScroll, {
      passive: true,
    })

    if (!this.props.isSnapshot) {
      document.addEventListener("selectionchange", this.handleSelectionChange, {
        passive: true,
      })
    }

    this.maybeExpandRenderWindow()
  }

  private maybeExpandRenderWindow() {
    if (this.renderWindowRafID) {
      cancelAnimationFrame(this.renderWindowRafID)
    }

    this.renderWindowRafID = requestAnimationFrame(() =>
      this.checkRenderWindow()
    )
  }

  private checkRenderWindow() {
    let blankWindowHeight = this.blankWindowHeight()
    if (this.pageYOffset >= blankWindowHeight) {
      return
    }

    let linesNeeded = Math.ceil(
      (blankWindowHeight - this.pageYOffset) / blankLogLineHeight
    )
    let step = Math.max(renderWindowMinStep, linesNeeded)
    let newRenderWindow = this.state.renderWindow + step
    if (this.state.renderWindow < newRenderWindow) {
      this.setState(prevState => {
        if (prevState.renderWindow < newRenderWindow) {
          return { renderWindow: newRenderWindow }
        }
        return null
      })
    }
  }

  componentDidUpdate(prevProps: LogPaneProps) {
    if (prevProps.manifestName != this.props.manifestName) {
      this.setState({ renderWindow: renderWindowDefault })
      this.autoscroll = true
      this.pageYOffset = -1
      if (this.props.isSnapshot) {
        this.autoscroll = false
      }

      this.scrollLastElementIntoView()
    } else if (this.autoscroll) {
      this.scrollLastElementIntoView()
    }

    this.maybeExpandRenderWindow()
  }

  scrollLastElementIntoView() {
    if (this.lastElement.current?.scrollIntoView) {
      this.lastElement.current.scrollIntoView()
    }
  }

  componentWillUnmount() {
    window.removeEventListener("scroll", this.onScroll)
    if (this.autoscrollRafID) {
      cancelAnimationFrame(this.autoscrollRafID)
    }
    if (this.renderWindowRafID) {
      cancelAnimationFrame(this.renderWindowRafID)
    }
    document.removeEventListener("selectionchange", this.handleSelectionChange)
  }

  handleSelectionChange() {
    let sel = document.getSelection()
    if (sel) {
      let node = ReactDOM.findDOMNode(this)
      if (!node) {
        return
      }

      let beginning = selection.startNode(sel)
      let end = selection.endNode(sel)
      if (
        !beginning ||
        !end ||
        !node.contains(beginning) ||
        !node.contains(end)
      ) {
        return
      }

      if (sel.isCollapsed) {
        this.props.handleClearHighlight()
      } else if (!node.isEqualNode(beginning) && !node.isEqualNode(end)) {
        let beginningLogLine = findLogLineID(beginning)
        let endingLogLine = findLogLineID(end)

        if (beginningLogLine && endingLogLine) {
          this.props.handleSetHighlight({
            beginningLogID: beginningLogLine,
            endingLogID: endingLogLine,
            text: selection.toString(),
          })
        }
      }
    }
  }

  private onScroll() {
    let pageYOffset = window.pageYOffset
    let oldPageYOffset = this.pageYOffset
    let autoscroll = this.autoscroll

    this.pageYOffset = pageYOffset
    if (oldPageYOffset === -1 || oldPageYOffset === pageYOffset) {
      return
    }

    // If we're scrolled horizontally, cancel the autoscroll.
    if (window.pageXOffset > 0) {
      this.autoscroll = false
      return
    }

    // If we're autoscrolling, and the user scrolled up,
    // cancel the autoscroll.
    if (autoscroll && pageYOffset < oldPageYOffset) {
      this.autoscroll = false
      return
    }

    // If we're not autoscrolling, and the user scrolled down,
    // we may have to re-engage the autoscroll.
    if (!autoscroll && pageYOffset > oldPageYOffset) {
      this.maybeEngageAutoscroll()
    }

    // If the user scrolled up, check to see if we've scrolled outside the render window.
    if (pageYOffset < oldPageYOffset) {
      this.maybeExpandRenderWindow()
    }
  }

  private renderWindowStart() {
    let lines = this.props.logLines
    let renderWindowStart = Math.max(0, lines.length - this.state.renderWindow)
    if (this.props.highlight && this.props.isSnapshot) {
      let highlightStart = Number(this.props.highlight.beginningLogID)
      if (!isNaN(highlightStart)) {
        renderWindowStart = Math.min(highlightStart, renderWindowStart)
      }
    }
    return renderWindowStart
  }

  private blankWindowHeight() {
    return blankLogLineHeight * this.renderWindowStart()
  }

  private maybeEngageAutoscroll() {
    if (this.props.isSnapshot) {
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
    let lines = this.props.logLines
    if (!lines || lines.length === 0) {
      return (
        <LogPaneRoot className="LogPane">
          <section className="Pane-empty-message">
            <LogoWordmarkSvg />
            <h2>No Logs Found</h2>
          </section>
        </LogPaneRoot>
      )
    }

    let logLineEls: Array<React.ReactElement> = []

    let sawBeginning = false
    let sawEnd = false
    let highlight = this.props.highlight
    let lastManifestName = ""
    let renderWindowStart = this.renderWindowStart()
    let blankWindowHeight = this.blankWindowHeight()
    logLineEls.push(
      <div key="blank" style={{ height: blankWindowHeight + "px" }}>
        &nbsp;
      </div>
    )

    for (let i = renderWindowStart; i < lines.length; i++) {
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
        <LogPaneLine
          ref={maybeHighlightRef}
          key={key}
          text={l.text}
          level={l.level}
          buildEvent={l.buildEvent}
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
      <LogEnd key="logEnd" className="logEnd" ref={this.lastElement}>
        &#9608;
      </LogEnd>
    )

    return <LogPaneRoot className="LogPane">{logLineEls}</LogPaneRoot>
  }
}

export default LogPane
