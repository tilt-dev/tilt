import React, { PureComponent } from "react"
import styled from "styled-components"
import { ReactComponent as CheckmarkSmallSvg } from "./assets/svg/checkmark-small.svg"
import { ReactComponent as ErrorSvg } from "./assets/svg/error.svg"
import { ReactComponent as WarningSvg } from "./assets/svg/warning.svg"
import { ClassNameFromResourceStatus } from "./ResourceStatus"
import {
  AnimDuration,
  Color,
  ColorAlpha,
  ColorRGBA,
  Glow,
  Width,
} from "./style-helpers"
import Tooltip from "./Tooltip"
import { ResourceStatus } from "./types"

type SidebarIconProps = {
  status: ResourceStatus
  alertCount: number
  tooltipText: string
}

let SidebarIconRoot = styled.div`
  display: flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  width: ${Width.statusIcon}px;
  margin-right: ${Width.statusIconMarginRight}px;
  transition: background-color ${AnimDuration.default} linear,
    opacity ${AnimDuration.default} linear;

  &.isWarning {
    background-color: ${Color.yellow};
  }
  &.isHealthy {
    background-color: ${Color.green};
  }
  &.isUnhealthy {
    background-color: ${Color.red};
  }
  &.isBuilding {
    background-color: ${ColorRGBA(Color.white, ColorAlpha.translucent)};
  }
  .isSelected &.isBuilding {
    background-color: ${ColorRGBA(Color.gray30, ColorAlpha.translucent)};
  }
  &.isPending {
    background-color: ${ColorRGBA(Color.white, ColorAlpha.translucent)};
    animation: ${Glow.white} 2s linear infinite;
  }
  .isSelected &.isPending {
    background-color: ${ColorRGBA(Color.gray30, ColorAlpha.translucent)};
    animation: ${Glow.dark} 2s linear infinite;
  }
  &.isNone {
    border-right: 1px solid ${Color.gray40};
    box-sizing: border-box;
    transition: border-color ${AnimDuration.default} linear;

    svg {
      fill: ${Color.gray50};
    }
  }
  .isSelected &.isNone {
    border-right-color: ${Color.grayLightest};

    svg {
      fill: ${Color.gray40};
    }
  }
`
// Sidenote: Resources with a `disabled` status are displayed without any icons,
// so that status case is not handled here
export default class SidebarIcon extends PureComponent<SidebarIconProps> {
  render() {
    let icon = <span>&nbsp;</span>
    if (this.props.status === ResourceStatus.Warning) {
      icon = (
        <WarningSvg
          fill={Color.white}
          width="10px"
          height="10px"
          role="presentation"
        />
      )
    } else if (this.props.status === ResourceStatus.Unhealthy) {
      icon = <ErrorSvg fill={Color.white} role="presentation" />
    } else if (this.props.status === ResourceStatus.None) {
      icon = <CheckmarkSmallSvg role="presentation" />
    }

    if (!this.props.tooltipText.length) {
      return (
        <SidebarIconRoot
          className={`${ClassNameFromResourceStatus(this.props.status)}`}
        >
          {icon}
        </SidebarIconRoot>
      )
    }
    return (
      <Tooltip title={this.props.tooltipText}>
        <SidebarIconRoot
          className={`${ClassNameFromResourceStatus(this.props.status)}`}
        >
          {icon}
        </SidebarIconRoot>
      </Tooltip>
    )
  }
}
