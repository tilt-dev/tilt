import React, { PureComponent, useContext } from "react"
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
import SidebarItem, { SidebarItemStyle } from "./SidebarItem"
import {
  SidebarPinButton,
  sidebarPinContext,
  SidebarPinContextProvider,
} from "./SidebarPin"
import SidebarKeyboardShortcuts from "./SidebarKeyboardShortcuts"
import { incr } from "./analytics"

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
let SidebarList = styled.span``
let SidebarItemBox = styled.span`
  color: ${Color.white};
  background-color: ${Color.gray};
  display: flex;
  align-items: stretch;
  float: right;
  width: ${Width.sidebar - Width.sidebarTriggerButton - 1}px;
  height: ${Height.sidebarItem}px;
  transition: color ${AnimDuration.default} linear,
    background-color ${AnimDuration.default} linear;
  border-radius: ${SizeUnit(0.15)};
  overflow: hidden;
  position: relative; // Anchor the .isBuilding::after psuedo-element

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
export const SidebarItemLink = styled(Link)`
  display: flex;
  align-items: stretch;
  text-decoration: none;
  // To truncate long names, root element needs an explicit width (i.e., not flex: 1)
  width: calc(100% - ${Width.sidebarTriggerButton}px);
`

let SidebarItemAll = styled(SidebarItemStyle)`
  margin-left: ${Width.sidebarPinButton}px;
  text-transform: uppercase;
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
let SidebarListSectionName = styled.span`
  float: right;
  width: ${Width.sidebar - Width.sidebarTriggerButton - 1}px;
  text-transform: uppercase;
  color: ${Color.grayLight};
  font-size: ${FontSize.small};
`
const SidebarListSectionItems = styled.ul`
  list-style: none;
`

export function SidebarListSection(
  props: React.PropsWithChildren<{ name: string }>
): JSX.Element {
  return (
    <span>
      <SidebarListSectionName>{props.name}</SidebarListSectionName>
      <SidebarListSectionItems>{props.children}</SidebarListSectionItems>
    </span>
  )
}

type Resource = Proto.webviewResource
type Build = Proto.webviewBuildRecord

export const triggerUpdate = (name: string, action: string): void => {
  incr("ui.web.triggerResource", { action })

  let url = `//${window.location.host}/api/trigger`

  fetch(url, {
    method: "post",
    body: JSON.stringify({
      manifest_names: [name],
      build_reason: 16 /* BuildReasonFlagTriggerWeb */,
    }),
  }).then(response => {
    if (!response.ok) {
      console.log(response)
    }
  })
}

type SidebarProps = {
  items: SidebarItem[]
  selected: string
  resourceView: ResourceView
  pathBuilder: PathBuilder
  initialPinnedItems?: Array<string>
}

function SidebarResources(props: SidebarProps) {
  return (
    <SidebarPinContextProvider initialValue={props.initialPinnedItems}>
      <PureSidebarResources {...props} />
    </SidebarPinContextProvider>
  )
}

function renderSidebarItem(
  item: SidebarItem,
  props: SidebarProps,
  renderPin: boolean
): JSX.Element {
  let link = `/r/${item.name}`
  switch (props.resourceView) {
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
  let buildDur = item.lastBuildDur ? formatBuildDuration(item.lastBuildDur) : ""
  let timeAgo = <TimeAgo date={item.lastDeployTime} formatter={formatter} />
  let isSelected = props.selected === item.name

  let isSelectedClass = isSelected ? "isSelected" : ""
  let isBuildingClass = building ? "isBuilding" : ""
  let onTrigger = triggerUpdate.bind(null, item.name)

  return (
    <SidebarItemStyle
      key={item.name}
      className={`${isSelectedClass} ${isBuildingClass}`}
    >
      {renderPin ? <SidebarPinButton resourceName={item.name} /> : null}
      <SidebarItemBox className={`${isSelectedClass} ${isBuildingClass}`}>
        <SidebarItemLink
          className="SidebarItem-link"
          to={props.pathBuilder.path(link)}
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
          isTiltfile={item.isTiltfile}
          isSelected={isSelected}
          hasPendingChanges={item.hasPendingChanges}
          hasBuilt={hasBuilt}
          isBuilding={building}
          triggerMode={item.triggerMode}
          isQueued={item.queued}
          onTrigger={onTrigger}
        />
      </SidebarItemBox>
    </SidebarItemStyle>
  )
}

function PinnedItems(props: SidebarProps) {
  let ctx = useContext(sidebarPinContext)
  let pinnedItems = ctx.pinnedResources?.flatMap(r =>
    props.items
      .filter(i => i.name === r)
      .map(i => renderSidebarItem(i, props, false))
  )

  if (!pinnedItems?.length) {
    return null
  }

  return <SidebarListSection name="favorites">{pinnedItems}</SidebarListSection>
}

// note: this is a PureComponent but we're not currently getting much value out of its pureness
// https://app.clubhouse.io/windmill/story/9949/web-purecomponent-optimizations-seem-to-not-be-working
class PureSidebarResources extends PureComponent<SidebarProps> {
  constructor(props: SidebarProps) {
    super(props)
    this.triggerSelected = this.triggerSelected.bind(this)
  }

  triggerSelected(action: string) {
    if (this.props.selected) {
      triggerUpdate(this.props.selected, action)
    }
  }

  render() {
    let pb = this.props.pathBuilder

    let allLink =
      this.props.resourceView === ResourceView.Alerts
        ? pb.path("/alerts")
        : pb.path("/")

    let totalAlerts = this.props.items
      .map(i => i.alertCount)
      .reduce((sum, current) => sum + current, 0)

    let listItems = this.props.items.map(item =>
      renderSidebarItem(item, this.props, true)
    )

    let nothingSelected = !this.props.selected

    return (
      <SidebarResourcesRoot className="Sidebar-resources">
        <SidebarList>
          <SidebarListSection name="">
            <SidebarItemAll>
              <SidebarItemBox className={nothingSelected ? "isSelected" : ""}>
                <SidebarItemLink to={allLink}>
                  <SidebarIcon
                    status={ResourceStatus.None}
                    alertCount={totalAlerts}
                  />
                  <SidebarItemName>All</SidebarItemName>
                </SidebarItemLink>
              </SidebarItemBox>
            </SidebarItemAll>
          </SidebarListSection>
          <PinnedItems {...this.props} />
          <SidebarListSection name="resources">{listItems}</SidebarListSection>
        </SidebarList>
        <SidebarKeyboardShortcuts
          selected={this.props.selected}
          items={this.props.items}
          pathBuilder={this.props.pathBuilder}
          onTrigger={this.triggerSelected}
        />
      </SidebarResourcesRoot>
    )
  }
}

export default SidebarResources
