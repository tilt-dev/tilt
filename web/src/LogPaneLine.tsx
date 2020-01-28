import {Fields} from "./types"
import styled from "styled-components"
import {SizeUnit, Width} from "./style-helpers"
import color from "./color"
import React, {PureComponent} from "react"
import AnsiLine from "./AnsiLine"

type LogPaneProps = {
  text: string
  manifestName: string
  level: string
  fields?: Fields | null
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
    if (props.fields?.progressID) {
      classes.push("is-progress")
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

export default LogPaneLine