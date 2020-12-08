import React, { PureComponent } from "react"
import styled, { keyframes } from "styled-components"
import {
  AnimDuration,
  Color,
  ColorAlpha,
  ColorRGBA,
  Font,
  FontSize,
  Width,
} from "./style-helpers"
import { ResourceStatus } from "./types"

type SidebarIconProps = {
  status: ResourceStatus
  alertCount: number
}

// For testing
export enum IconType {
  StatusDefault = "default",
  StatusPending = "pending",
  StatusBuilding = "building",
}

let glowWhite = keyframes`
  0% {
    background-color: ${ColorRGBA(Color.white, ColorAlpha.translucent)};
  }
  50% {
    background-color: ${ColorRGBA(Color.white, ColorAlpha.almostTransparent)};
  }
`

let glowDark = keyframes`
  0% {
    background-color: ${ColorRGBA(Color.gray, ColorAlpha.translucent)};
  }
  50% {
    background-color: ${ColorRGBA(Color.gray, ColorAlpha.almostTransparent)};
  }
`

let SidebarIconRoot = styled.div`
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  width: ${Width.sidebarCollapsed * 0.7}px;
  margin-right: ${Width.sidebarCollapsed * 0.3}px;
  transition: background-color ${AnimDuration.default} linear,
    opacity ${AnimDuration.default} linear;

  &.isWarning {
    background:
		  linear-gradient(0deg, transparent 0, transparent 6px, ${Color.yellow} 6px, ${Color.yellow} 7px),
		  linear-gradient(90deg, transparent 0, transparent 1px, 
                             ${Color.yellow} 1px, ${Color.yellow} 2px),
		  ${Color.yellowLight};
	  background-size: 2px 7px;
  }
  &.isHealthy {
    background-color: ${Color.green};
  }
  &.isUnhealthy {
    background:
		  radial-gradient(circle at 50% 50%, ${Color.redDark} 0, ${Color.redDark} 0.65px, transparent 0.65px),
      ${Color.red};
	  background-size: 3px 3px;
  }
  &.isBuilding {
    background-color: ${ColorRGBA(Color.white, ColorAlpha.translucent)};
  }
  .isSelected &.isBuilding {
    background-color: ${ColorRGBA(Color.gray, ColorAlpha.translucent)};
  }
  &.isPending {
    background-color: ${ColorRGBA(Color.white, ColorAlpha.translucent)};
    animation: ${glowWhite} 2s linear infinite;
  }
  .isSelected &.isPending {
    background-color: ${ColorRGBA(Color.gray, ColorAlpha.translucent)};
    animation: ${glowDark} 2s linear infinite;
  }
  &.isNone {
    background-color: ${ColorRGBA(Color.white, ColorAlpha.translucent)};
  }
  .isSelected &.isNone {
    background-color: ${ColorRGBA(Color.gray, ColorAlpha.translucent)};
  }
`

let AlertCount = styled.span`
  font-family: ${Font.sansSerif};
  font-size: ${FontSize.smallest};
  color: ${Color.black};
`

export default class SidebarIcon extends PureComponent<SidebarIconProps> {
  render() {
    return (
      <SidebarIconRoot className={`${this.status()}`}>
        {this.props.alertCount > 0 ? (
          <AlertCount>{this.props.alertCount}</AlertCount>
        ) : (
          <span>&nbsp;</span>
        )}
      </SidebarIconRoot>
    )
  }

  status() {
    switch (this.props.status) {
      case ResourceStatus.Building:
        return "isBuilding"
      case ResourceStatus.Pending:
        return "isPending"
      case ResourceStatus.Warning:
        return "isWarning"
      case ResourceStatus.Healthy:
        return "isHealthy"
      case ResourceStatus.Unhealthy:
        return "isUnhealthy"
      case ResourceStatus.None:
        return "isNone"
    }
  }
}
