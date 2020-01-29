import {Fields} from "./types"
import React, {PureComponent} from "react"
import AnsiLine from "./AnsiLine"
import {Color, ColorRGBA, ColorAlpha, SizeUnit, Width} from "./style-helpers"
import styled from "styled-components"
import Ansi from "ansi-to-react"

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

let LogPaneLineRoot = styled.span`
  display: flex;
 
  &.is-highlighted {
    background-color: ${ColorRGBA(Color.blue, ColorAlpha.translucent)};
  }
`
let LogLinePrefixRoot = styled.span`
  user-select: none;
  width: ${Width.secondaryNavItem}px; // Match height of tab above
  box-sizing: border-box;
  background-color: ${Color.grayDark};
  color: ${Color.grayLightest};
  padding-left: ${SizeUnit(0.5)};
  padding-right: ${SizeUnit(0.5)};
  text-align: right;
  flex-shrink: 0;
  // Truncate long text:
  text-overflow: ellipsis;
  overflow: hidden;
  white-space: nowrap;
  
  ${LogPaneLineRoot}.is-contextChange > & {
    margin-top: -1px;
    border-top: 1px dotted ${Color.grayLightest};
  }
`

let LineContent = styled(AnsiLine)`
  white-space: pre-wrap;
  padding-left: ${SizeUnit(0.6)};
  flex: 1;
  
  ${LogLinePrefixRoot} + & {
    border-left: 1px solid ${Color.grayLight};
  }
  
  ${LogPaneLineRoot}.is-warning & {
    border-left: ${Width.logLineGutter}px solid ${Color.yellow};
  }
  ${LogPaneLineRoot}.is-error & {
    border-left: ${Width.logLineGutter}px solid ${Color.red};
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
    let classes = []
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
    if (props.fields?.progressID) {
      classes.push("is-progress")
    }
    return (
      <LogPaneLineRoot
        ref={this.ref}
        data-lineid={props.lineId}
        className={classes.join(" ")}
      >
        {prefix}
        <LineContent line={text} />
      </LogPaneLineRoot>
    )
  }
}

export default LogPaneLine