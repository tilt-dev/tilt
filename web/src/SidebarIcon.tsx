import React, { PureComponent } from "react"
import { TriggerMode, ResourceStatus, Build } from "./types"
import { Color } from "./constants"
import { ReactComponent as ManualSvg } from "./assets/svg/indicator-manual.svg"
import { ReactComponent as ManualBuildingSvg } from "./assets/svg/indicator-manual-building.svg"
import { ReactComponent as AutoSvg } from "./assets/svg/indicator-auto.svg"
import { ReactComponent as AutoPendingSvg } from "./assets/svg/indicator-auto-pending.svg"
import { ReactComponent as AutoBuildingSvg } from "./assets/svg/indicator-auto-building.svg"

type SidebarIconProps = {
  triggerMode: TriggerMode
  status: ResourceStatus
  hasWarning: boolean
  isBuilding: boolean
  isDirty: boolean
  lastBuild: Build | null
}

export enum IconType {
  DotAuto = "dotAuto",
  DotAutoPending = "dotAutoPending",
  DotAutoBuilding = "dotAutoBuilding",
  DotManual = "dotManual",
  DotManualPending = "dotManualPending",
  DotManualBuilding = "dotManualBuilding",
}

export default class SidebarIcon extends PureComponent<SidebarIconProps> {
  render() {
    let props = this.props
    let fill = Color.green

    if (props.status === ResourceStatus.Error) {
      fill = Color.red
    } else if (props.hasWarning) {
      fill = Color.yellow
    } else if (props.isDirty && props.lastBuild && props.lastBuild.Error) {
      fill = Color.red
    }

    if (props.triggerMode === TriggerMode.TriggerModeManual) {
      return this.renderManual(fill)
    }

    return this.renderAuto(fill)
  }

  renderAuto(fill: Color) {
    let props = this.props
    if (props.isBuilding) {
      return this.dotAutoBuilding()
    }

    if (props.status === ResourceStatus.Pending) {
      return this.dotAutoPending()
    }

    return this.dotAuto(fill)
  }

  renderManual(fill: Color) {
    let props = this.props
    if (props.isBuilding) {
      return this.dotManualBuilding()
    }

    if (props.isDirty) {
      return this.dotManual(fill)
    }

    if (props.status === ResourceStatus.Pending) {
      return this.dotManualPending()
    }

    return this.dotManual(fill)
  }

  dotAuto(fill: Color) {
    return <AutoSvg className={`${IconType.DotAuto} auto`} fill={fill} />
  }

  dotAutoPending() {
    let style = {
      animation: "glow 1s linear infinite",
    }
    return (
      <AutoPendingSvg
        className={`${IconType.DotAutoPending} auto`}
        style={style}
        stroke={Color.gray}
      />
    )
  }

  dotAutoBuilding() {
    let style = {
      animation: "spin 1s linear infinite",
    }
    return (
      <AutoBuildingSvg
        className={`${IconType.DotAutoBuilding} auto`}
        fill={Color.gray}
        style={style}
      />
    )
  }

  dotManual(fill: Color) {
    return <ManualSvg className={`${IconType.DotManual} manual`} fill={fill} />
  }

  dotManualPending() {
    let style = {
      animation: "glow 1s linear infinite",
    }

    return (
      <ManualSvg
        className={`${IconType.DotManualPending} manual`}
        style={style}
        fill={Color.gray}
      />
    )
  }

  dotManualBuilding() {
    let style = {
      animation: "spin 1s linear infinite",
    }

    return (
      <ManualBuildingSvg
        className={`${IconType.DotManualBuilding} manual`}
        fill={Color.gray}
        style={style}
      />
    )
  }
}
