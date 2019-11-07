import React, { PureComponent } from "react"
import { Build, ResourceStatus } from "./types"
import { Color } from "./constants"
import { ReactComponent as DotSvg } from "./assets/svg/indicator-auto.svg"
import { ReactComponent as DotPendingSvg } from "./assets/svg/indicator-auto-pending.svg"
import { ReactComponent as DotBuildingSvg } from "./assets/svg/indicator-auto-building.svg"
import "./SidebarIcon.scss"

type SidebarIconProps = {
  status: ResourceStatus
}

// For testing
export enum IconType {
  StatusDefault = "default",
  StatusPending = "pending",
  StatusBuilding = "building",
}

export default class SidebarIcon extends PureComponent<SidebarIconProps> {
  render() {
    return <div className="SidebarIcon">{this.svg()}</div>
  }

  svg() {
    switch (this.props.status) {
      case ResourceStatus.Building:
        return this.building()
      case ResourceStatus.Pending:
        return this.pending()
      case ResourceStatus.Warning:
        return this.default(Color.yellow)
      case ResourceStatus.Healthy:
        return this.default(Color.green)
      case ResourceStatus.Unhealthy:
        return this.default(Color.red)
      case ResourceStatus.None:
        return this.default(Color.gray)
    }
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
