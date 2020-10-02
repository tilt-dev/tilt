import React, { PureComponent } from "react"
import { Link } from "react-router-dom"
import { ResourceStatus, ResourceView } from "./types"
import TimeAgo from "react-timeago"
import { timeAgoFormatter } from "./timeFormatters"
import { isZeroTime } from "./time"
import PathBuilder from "./PathBuilder"
import SidebarIcon from "./SidebarIcon"
import SidebarTriggerButton from "./SidebarTriggerButton"
import styled, { keyframes } from "styled-components"
import { formatBuildDuration } from "./format"
import {
  AnimDuration,
  Color,
  ColorAlpha,
  ColorRGBA,
  FontSize,
  Height,
  SizeUnit,
  Width,
} from "./style-helpers"
import SidebarItem from "./SidebarItem"

// Styles:
const barberpole = keyframes`
  100% {
    background-position: 100% 100%;
  }
`
let SidebarResourcesRoot = styled.nav`
  flex: 1 0 auto;
  margin-left: ${SizeUnit(0.2)};
  margin-right: ${SizeUnit(0.2)};
`
let SidebarList = styled.ul`
  list-style: none;
`
let SidebarItemStyle = styled.li`
  width: 100%;
  color: ${Color.white};
  background-color: ${Color.gray};
  display: flex;
  align-items: stretch;
  height: ${Height.sidebarItem}px;
  transition: color ${AnimDuration.default} linear,
    background-color ${AnimDuration.default} linear;
  border-radius: ${SizeUnit(0.15)};
  overflow: hidden;
  position: relative; // Anchor the .isBuilding::after psuedo-element

  & + & {
    margin-top: ${SizeUnit(0.2)};
  }

  &:hover {
    background-color: ${ColorRGBA(Color.gray, ColorAlpha.translucent)};
    color: ${Color.blue};
  }

  &.isSelected {
    background-color: ${Color.white};
    color: ${Color.gray};
  }

  &.isBuilding::after {
    content: "";
    position: absolute;
    pointer-events: none;
    width: 100%;
    top: 0;
    bottom: 0;
    background: repeating-linear-gradient(
      225deg,
      ${ColorRGBA(Color.grayLight, ColorAlpha.translucent)},
      ${ColorRGBA(Color.grayLight, ColorAlpha.translucent)} 1px,
      ${ColorRGBA(Color.black, 0)} 1px,
      ${ColorRGBA(Color.black, 0)} 6px
    );
    background-size: 200% 200%;
    animation: ${barberpole} 8s linear infinite;
  }
`
let SidebarItemLink = styled(Link)`
  display: flex;
  align-items: stretch;
  text-decoration: none;
  // To truncate long names, root element needs an explicit width (i.e., not flex: 1)
  width: calc(100% - ${Width.sidebarTriggerButton}px);
`

let SidebarItemAll = styled(SidebarItemStyle)`
  text-transform: uppercase;
  margin-top: ${SizeUnit(0.5)};
  margin-bottom: ${SizeUnit(0.2)};
`

let SidebarItemName = styled.p`
  color: inherit;
  display: flex;
  align-items: center;
  flex: 1;
  overflow: hidden; // Reinforce truncation
`

// This child element helps truncated names show ellipses properly:
let SidebarItemNameTruncate = styled.span`
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
`

let SidebarTiming = styled.div`
  font-size: ${FontSize.small};
  line-height: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: flex-end;
  flex-basis: auto;
`
let SidebarItemDuration = styled.span`
  opacity: ${ColorAlpha.almostOpaque};
  color: inherit;
`
let SidebarItemTimeAgo = styled.span`
  color: inherit;
`

type Resource = Proto.webviewResource
type Build = Proto.webviewBuildRecord

type SidebarProps = {
  items: SidebarItem[]
  selected: string
  resourceView: ResourceView
  pathBuilder: PathBuilder
}

class SidebarResources extends PureComponent<SidebarProps> {
  render() {
    let pb = this.props.pathBuilder

    let allLink =
      this.props.resourceView === ResourceView.Alerts
        ? pb.path("/alerts")
        : pb.path("/")

    let totalAlerts = this.props.items
      .map(i => i.alertCount)
      .reduce((sum, current) => sum + current, 0)

    let listItems = this.props.items.map(item => {
      let link = `/r/${item.name}`
      switch (this.props.resourceView) {
        case ResourceView.Alerts:
          link += "/alerts"
          break
        case ResourceView.Facets:
          link += "/facets"
          break
      }

      let formatter = timeAgoFormatter
      let hasSuccessfullyDeployed = !isZeroTime(item.lastDeployTime)
      let hasBuilt = item.lastBuild !== null
      let building = !isZeroTime(item.currentBuildStartTime)
      let buildDur = item.lastBuildDur
        ? formatBuildDuration(item.lastBuildDur)
        : ""
      let timeAgo = <TimeAgo date={item.lastDeployTime} formatter={formatter} />
      let isSelected = this.props.selected === item.name

      let isSelectedClass = isSelected ? "isSelected" : ""
      let isBuildingClass = building ? "isBuilding" : ""

      return (
        <SidebarItemStyle
          key={item.name}
          className={`${isSelectedClass} ${isBuildingClass}`}
        >
          <SidebarItemLink
            className="SidebarItem-link"
            to={pb.path(link)}
            title={item.name}
          >
            <SidebarIcon status={item.status} alertCount={item.alertCount} />
            <SidebarItemName>
              <SidebarItemNameTruncate>{item.name}</SidebarItemNameTruncate>
            </SidebarItemName>
            <SidebarTiming>
              <SidebarItemTimeAgo
                className={hasSuccessfullyDeployed ? "" : "isEmpty"}
              >
                {hasSuccessfullyDeployed ? timeAgo : "—"}
              </SidebarItemTimeAgo>
              <SidebarItemDuration
                className={hasSuccessfullyDeployed ? "" : "isEmpty"}
              >
                {hasSuccessfullyDeployed ? buildDur : "—"}
              </SidebarItemDuration>
            </SidebarTiming>
          </SidebarItemLink>
          <SidebarTriggerButton
            resourceName={item.name}
            isTiltfile={item.isTiltfile}
            isSelected={isSelected}
            hasPendingChanges={item.hasPendingChanges}
            hasBuilt={hasBuilt}
            isBuilding={building}
            triggerMode={item.triggerMode}
            isQueued={item.queued}
          />
        </SidebarItemStyle>
      )
    })

    let nothingSelected = !this.props.selected

    return (
      <SidebarResourcesRoot className="Sidebar-resources">
        <SidebarList>
          <SidebarItemAll className={nothingSelected ? "isSelected" : ""}>
            <SidebarItemLink to={allLink}>
              <SidebarIcon
                status={ResourceStatus.None}
                alertCount={totalAlerts}
              />
              <SidebarItemName>All</SidebarItemName>
            </SidebarItemLink>
          </SidebarItemAll>
          {listItems}
        </SidebarList>
      </SidebarResourcesRoot>
    )
  }
}

export default SidebarResources
