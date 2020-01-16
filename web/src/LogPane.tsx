import React, { Component, PureComponent } from "react"
import { ReactComponent as LogoWordmarkSvg } from "./assets/svg/logo-wordmark-gray.svg"
import AnsiLine from "./AnsiLine"
import "./LogPane.scss"
import ReactDOM from "react-dom"
import { LogLine, SnapshotHighlight } from "./types"
import color from "./color"
import { SizeUnit, Width } from "./style-helpers"
import findLogLineID from "./findLogLine"
import styled from "styled-components"

type LogPaneProps = {
  manifestName: string
  logLines: LogLine[]
  showManifestPrefix: boolean
  message?: string
  handleSetHighlight: (highlight: SnapshotHighlight) => void
  handleClearHighlight: () => void
  highlight: SnapshotHighlight | null | undefined
  modalIsOpen: boolean
  isSnapshot: boolean
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
    ${Width.secondaryNavItem}px - ${SizeUnit(0.5)}
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
    } else if (props.level == "ERROR") {
      classes.push("is-error")
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

class LogPane extends Component<LogPaneProps> {
  highlightRef: React.RefObject<LogLineComponent> = React.createRef()
  private lastElement: React.RefObject<HTMLParagraphElement> = React.createRef()
  private rafID: number | null = null

  // Whether we're auto-scrolling to the bottom of the screen.
  private autoscroll: boolean

  // Track the pageYOffset to see if the user is scrolling upwards.
  private pageYOffset: number

  constructor(props: LogPaneProps) {
    super(props)

    this.autoscroll = true
    this.pageYOffset = -1

    this.onScroll = this.onScroll.bind(this)
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
      window.addEventListener("scroll", this.onScroll, {
        passive: true,
      })
      document.addEventListener("selectionchange", this.handleSelectionChange, {
        passive: true,
      })
    }
  }

  componentDidUpdate(prevProps: LogPaneProps) {
    if (prevProps.manifestName != this.props.manifestName) {
      this.autoscroll = true
      this.pageYOffset = -1

      this.scrollLastElementIntoView()
    } else if (this.autoscroll) {
      this.scrollLastElementIntoView()
    }
  }

  scrollLastElementIntoView() {
    if (this.lastElement.current?.scrollIntoView) {
      this.lastElement.current.scrollIntoView()
    }
  }

  componentWillUnmount() {
    window.removeEventListener("scroll", this.onScroll)
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
  }

  maybeEngageAutoscroll() {
    if (this.rafID) {
      cancelAnimationFrame(this.rafID)
    }

    this.rafID = requestAnimationFrame(() => {
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
