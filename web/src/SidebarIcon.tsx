import React, { PureComponent } from "react"
import { TriggerMode, RuntimeStatus } from "./types"
import { Color } from "./constants"

type SidebarIconProps = {
  triggerMode: TriggerMode
  status: RuntimeStatus
  hasWarning: boolean
  isBuilding: boolean
}

export enum IconType {
  DotAuto = "dotAuto",
  DotAutoPending = "dotAutoPending",
  DotAutoBuilding = "dotAutoBuilding",
  DotManual = "dotManual",
  DotManualBuilding = "dotManualBuilding",
}

export default class SidebarIcon extends PureComponent<SidebarIconProps> {
  render() {
    let props = this.props
    let fill = Color.green
    if (props.status === RuntimeStatus.Pending) {
      fill = Color.gray
    } else if (props.status === RuntimeStatus.Error) {
      fill = Color.red
    } else if (props.hasWarning) {
      fill = Color.yellow
    }

    if (props.triggerMode === TriggerMode.TriggerModeManual) {
      return this.renderManual(fill)
    }

    return this.renderAuto(fill)
  }

  renderAuto(fill: Color) {
    let props = this.props
    if (props.isBuilding) {
      return this.dotAutoBuilding(fill)
    }

    if (props.status === RuntimeStatus.Pending) {
      return this.dotAutoPending(fill)
    }

    return this.dotAuto(fill)
  }

  renderManual(fill: Color) {
    let props = this.props
    if (props.isBuilding) {
      return this.dotManualBuilding(fill)
    }

    return this.dotManualPending(fill)
  }

  dotAuto(fill: Color) {
    return (
      <svg
        className={`${IconType.DotAuto} auto`}
        height="10"
        viewBox="0 0 10 10"
        width="10"
        xmlns="http://www.w3.org/2000/svg"
        fill={fill}
      >
        <rect fillRule="evenodd" height="10" rx="5" width="10" />
      </svg>
    )
  }

  dotAutoPending(fill: Color) {
    let style = {
      animation: "glow 1s linear infinite",
    }
    return (
      <svg
        className={`${IconType.DotAutoPending} auto`}
        height="10"
        viewBox="0 0 10 10"
        width="10"
        xmlns="http://www.w3.org/2000/svg"
        style={style}
      >
        <rect
          height="7.988"
          rx="3.994"
          stroke={Color.gray}
          strokeWidth="2"
          width="8"
          x="1"
          y="1"
        />
      </svg>
    )
  }

  dotAutoBuilding(fill: Color) {
    let style = {
      animation: "spin 1s linear infinite",
    }
    return (
      <svg
        className={`${IconType.DotAutoBuilding} auto`}
        height="10"
        viewBox="0 0 10 10"
        width="10"
        xmlns="http://www.w3.org/2000/svg"
        fill={fill}
        style={style}
      >
        <path
          d="m9.65354132 6.83246601-1.85421206-.75144721c.12960419-.33538852.20067074-.69990442.20067074-1.0810188 0-1.65685425-1.34314575-3-3-3s-3 1.34314575-3 3 1.34314575 3 3 3c.25896905 0 .51027414-.03281333.74998437-.09450909l.37116491 1.96832235c-.360418.08256902-.73568991.12618674-1.12114928.12618674-2.76142375 0-5-2.23857625-5-5s2.23857625-5 5-5 5 2.23857625 5 5c0 .64686359-.12283756 1.26503693-.34645868 1.83246601z"
          fillRule="evenodd"
        />
      </svg>
    )
  }

  dotManualPending(fill: Color) {
    return (
      <svg
        className={`${IconType.DotManual} manual`}
        height="24"
        viewBox="0 0 24 24"
        width="24"
        xmlns="http://www.w3.org/2000/svg"
      >
        <path
          d="m12 24c-6.627417 0-12-5.372583-12-12s5.372583-12 12-12 12 5.372583 12 12-5.372583 12-12 12zm0-3c4.9705627 0 9-4.0294373 9-9 0-4.97056275-4.0294373-9-9-9-4.97056275 0-9 4.02943725-9 9 0 4.9705627 4.02943725 9 9 9z"
          fillRule="evenodd"
          fill={fill}
        />
      </svg>
    )
  }

  dotManualBuilding(fill: Color) {
    return (
      <svg
        className={`${IconType.DotManualBuilding} manual`}
        height="24"
        viewBox="0 0 24 24"
        width="24"
        xmlns="http://www.w3.org/2000/svg"
      >
        <path
          d="m24 12h-3c0-4.97056275-4.0294373-9-9-9-4.97056275 0-9 4.02943725-9 9 0 4.9705627 4.02943725 9 9 9v3c-6.627417 0-12-5.372583-12-12s5.372583-12 12-12 12 5.372583 12 12z"
          fill={fill}
          fillRule="evenodd"
        />
      </svg>
    )
  }
}
