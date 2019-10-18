import React, { PureComponent } from "react"
import { TriggerMode, RuntimeStatus, Build } from "./types"
import { Color } from "./constants"
import { ReactComponent as DotSvg } from "./assets/svg/indicator-auto.svg"
import { ReactComponent as DotPendingSvg } from "./assets/svg/indicator-auto-pending.svg"
import { ReactComponent as DotBuildingSvg } from "./assets/svg/indicator-auto-building.svg"
import "./SidebarIcon.scss"

type SidebarIconProps = {
  status: RuntimeStatus
  hasWarning: boolean
  isBuilding: boolean
  isDirty: boolean
  lastBuild: Build | null
}

// For testing
export enum IconType {
  StatusDefault = "default",
  StatusPending = "pending",
  StatusBuilding = "building",
}

export default class SidebarIcon extends PureComponent<SidebarIconProps> {
  render() {
    let props = this.props
    let fill = Color.green
    let dirtyBuildWithError =
      props.isDirty && props.lastBuild && props.lastBuild.error

    if (props.status === RuntimeStatus.Error) {
      fill = Color.red
    } else if (props.hasWarning) {
      fill = Color.yellow
    } else if (dirtyBuildWithError) {
      fill = Color.red
    }

    return <div className="SidebarIcon">{this.renderSvg(fill)}</div>
  }

  renderSvg(fill: Color) {
    let props = this.props
    if (props.isBuilding) {
      return this.building()
    }

    if (props.status === RuntimeStatus.Pending) {
      return this.pending()
    }

    return this.default(fill)
  }

  default(fill: Color) {
    return <DotSvg className={IconType.StatusDefault} fill={fill} />
  }

  pending() {
    let style = {
      animation: "glow 1s linear infinite",
    }
    return <DotPendingSvg className={IconType.StatusPending} style={style} />
  }

  building() {
    let style = {
      animation: "spin 1s linear infinite",
    }
    return <DotBuildingSvg className={IconType.StatusBuilding} style={style} />
  }
}
