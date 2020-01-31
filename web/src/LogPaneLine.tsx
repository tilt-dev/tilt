import {Fields, BuildEvent} from "./types"
import React, { PureComponent } from "react"
import AnsiLine from "./AnsiLine"
import {
  Color,
  ColorRGBA,
  ColorAlpha,
  SizeUnit,
  Height,
  Width,
} from "./style-helpers"
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

  &.is-buildEvent-init {
    margin-top: ${SizeUnit(0.5)};
    margin-bottom: ${SizeUnit(0.5)};
    background-color: ${Color.gray};
    text-align: right;
    padding-right: ${SizeUnit(1)};
    border-top: 1px solid ${ColorRGBA(Color.grayLight, ColorAlpha.translucent)};
    border-bottom: 1px solid ${ColorRGBA(Color.grayLight, ColorAlpha.translucent)};
  }
  &.is-buildEvent-fallback {
    background-color: ${Color.grayDarker};
    
    // A lil' trick so bottom margin only appears after the last element
    margin-top: -${SizeUnit(0.5)};
    margin-bottom: ${SizeUnit(0.5)};
  }
`
let LogLinePrefixRoot = styled.span`
  user-select: none;
  width: ${Width.secondaryNavItem}px; // Match height of tab above
  box-sizing: border-box;
  background-color: ${Color.grayDarker};
  color: ${Color.grayLightest};
  padding-left: ${SizeUnit(0.5)};
  padding-right: ${SizeUnit(0.5)};
  display: flex;
  align-items: center;
  justify-content: right;
  flex-shrink: 0;
  // Truncate long text:
  text-overflow: ellipsis;
  overflow: hidden;
  white-space: nowrap;

  ${LogPaneLineRoot}.is-contextchange > & {
    margin-top: -${Height.logLineSeparator}px;
    border-top: ${Height.logLineSeparator}px solid ${Color.gray};
  }
`

let LineContent = styled(AnsiLine)`
  white-space: pre-wrap;
  padding-left: ${SizeUnit(0.6)};
  flex: 1;
  
  // Make the layout around the text a bit more generous
  ${LogPaneLineRoot}.is-buildEvent-init &,
  ${LogPaneLineRoot}.is-buildEvent-fallback & {
    padding-top: ${SizeUnit(0.2)};
    padding-bottom: ${SizeUnit(0.2)};
  }

  // A left border draws your eye to notable logs. 
  // It's to the right of the prefix, so its position is always near the log.
  ${LogPaneLineRoot}.is-warning & {
    border-left: ${Width.logLineGutter}px solid ${Color.yellow};
  }
  ${LogPaneLineRoot}.is-error & {
    border-left: ${Width.logLineGutter}px solid ${Color.red};
  }
  ${LogPaneLineRoot}.is-buildEvent-fallback & {
    border-left: ${Width.logLineGutter}px solid ${Color.blueDark};
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
    let classes = ["logPaneLine"]
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
    if (props.fields?.buildEvent == BuildEvent.Init) {
      classes.push("is-buildEvent-init")
    }
    if (props.fields?.buildEvent == BuildEvent.Fallback) {
      classes.push("is-buildEvent-fallback")
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
