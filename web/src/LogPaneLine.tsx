import React, { PureComponent } from "react"
import AnsiLine from "./AnsiLine"
import "./LogPaneLine.scss"

type LogPaneProps = {
  text: string
  manifestName: string
  level: string
  buildEvent?: string
  lineId: number
  shouldHighlight: boolean
  showManifestPrefix: boolean
  isContextChange: boolean
}

let LogLinePrefix = React.memo((props: { name: string }) => {
  let name = props.name
  if (!name) {
    name = "(global)"
  }
  return (
    <span className="logLinePrefix" title={name}>
      {name}
    </span>
  )
})

class LogPaneLine extends PureComponent<LogPaneProps> {
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
    let classes = ["LogPaneLine"]
    if (props.shouldHighlight) {
      classes.push("is-highlighted")
    }
    if (props.level == "WARN") {
      classes.push("is-warning")
    } else if (props.level == "ERROR") {
      classes.push("is-error")
    }
    if (props.isContextChange) {
      classes.push("is-contextChange")
    }
    if (props.buildEvent == "init") {
      classes.push("is-buildEvent-init")
    }
    if (props.buildEvent == "fallback") {
      classes.push("is-buildEvent-fallback")
    }

    return (
      <span
        ref={this.ref}
        data-lineid={props.lineId}
        className={classes.join(" ")}
      >
        {prefix}
        <AnsiLine className="LogPaneLine-content" line={text} />
      </span>
    )
  }
}

export default LogPaneLine
