import React, { PureComponent, useContext } from "react"
import { Link } from "react-router-dom"
import TimeAgo from "react-timeago"
import styled, { keyframes } from "styled-components"
import { incr } from "./analytics"
import { formatBuildDuration } from "./format"
import PathBuilder from "./PathBuilder"
import SidebarIcon from "./SidebarIcon"
import SidebarItem, { SidebarItemRoot } from "./SidebarItem"
import SidebarKeyboardShortcuts from "./SidebarKeyboardShortcuts"
import {
  SidebarPinButton,
  SidebarPinButtonSpacer,
  sidebarPinContext,
  SidebarPinContextProvider,
} from "./SidebarPin"
import SidebarTriggerButton from "./SidebarTriggerButton"
import {
  AnimDuration,
  Color,
  ColorAlpha,
  ColorRGBA,
  Font,
  FontSize,
  SizeUnit,
  Width,
} from "./style-helpers"
import { isZeroTime } from "./time"
import { timeAgoFormatter } from "./timeFormatters"
import { ResourceStatus, ResourceView } from "./types"

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
let SidebarList = styled.div``

export let SidebarItemBox = styled(Link)`
  color: ${Color.white};
  background-color: ${Color.gray};
  display: flex;
  flex-direction: column;
  transition: color ${AnimDuration.default} linear,
    background-color ${AnimDuration.default} linear;
  border-radius: 5px;
  overflow: hidden;
  border: 1px solid ${Color.grayLighter};
  position: relative; // Anchor the .isBuilding::after psuedo-element
  flex-grow: 1;
  text-decoration: none;
  font-size: ${FontSize.small};
  font-family: ${Font.monospace};
  margin-right: ${SizeUnit(0.5)};

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

let SidebarItemAllBox = styled(SidebarItemBox)`
  flex-direction: row;
  height: ${SizeUnit(1.0)};
`

let SidebarItemRuntimeBox = styled.div`
  display: flex;
  align-items: center;
  height: ${SizeUnit(1)};
  border-bottom: 1px solid ${Color.grayLighter};
  box-sizing: border-box;
  transition: border-color ${AnimDuration.default} linear;

  .isSelected & {
    border-bottom-color: ${Color.grayLightest};
  }
`

let SidebarItemBuildBox = styled.div`
  display: flex;
  align-items: center;
  flex-shrink: 1;
  height: ${SizeUnit(0.875)};
`

let SidebarItemAllRoot = styled(SidebarItemRoot)`
  margin-left: ${Width.sidebarPinButton}px;
  text-transform: uppercase;
`

let SidebarItemText = styled.div`
  display: flex;
  align-items: center;
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  opacity: ${ColorAlpha.almostOpaque};
  line-height: normal;
`

let SidebarItemAllName = styled(SidebarItemText)`
  opacity: 1;
`

type SidebarItemAllProps = {
  nothingSelected: boolean
  totalAlerts: number
  allLink: string
}

export function SidebarItemAll(props: SidebarItemAllProps) {
  return (
    <SidebarItemAllRoot>
      <SidebarItemAllBox
        className={props.nothingSelected ? "isSelected" : ""}
        to={props.allLink}
      >
        <SidebarIcon
          status={ResourceStatus.None}
          alertCount={props.totalAlerts}
        />
        <SidebarItemAllName>All</SidebarItemAllName>
      </SidebarItemAllBox>
    </SidebarItemAllRoot>
  )
}

let SidebarItemNameRoot = styled(SidebarItemText)`
  opacity: 1;
  font-family: ${Font.sansSerif};
  z-index: 1; // Appear above the .isBuilding gradient
`

let SidebarItemNameTruncate = styled.span`
  overflow: hidden;
  text-overflow: ellipsis;
`

let SidebarItemName = (props: { children: React.ReactNode }) => {
  return (
    <SidebarItemNameRoot>
      <SidebarItemNameTruncate>{props.children}</SidebarItemNameTruncate>
    </SidebarItemNameRoot>
  )
}

let SidebarItemTimeAgo = styled.span`
  opacity: ${ColorAlpha.almostOpaque};
`

let SidebarListSectionName = styled.div`
  width: ${Width.sidebar - Width.sidebarTriggerButton - 1}px;
  margin-left: ${Width.sidebarPinButton}px;
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
    <div>
      <SidebarListSectionName>{props.name}</SidebarListSectionName>
      <SidebarListSectionItems>{props.children}</SidebarListSectionItems>
    </div>
  )
}

type Resource = Proto.webviewResource
type Build = Proto.webviewBuildRecord

export function triggerUpdate(name: string, action: string) {
  incr("ui.web.triggerResource", { action })

  let url = `//${window.location.host}/api/trigger`

  fetch(url, {
    method: "post",
    body: JSON.stringify({
      manifest_names: [name],
      build_reason: 16 /* BuildReasonFlagTriggerWeb */,
    }),
  }).then((response) => {
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
  initialPinnedItemsForTesting?: string[]
}

function SidebarResources(props: SidebarProps) {
  return (
    <SidebarPinContextProvider
      initialValueForTesting={props.initialPinnedItemsForTesting}
    >
      <PureSidebarResources {...props} />
    </SidebarPinContextProvider>
  )
}

export type SidebarItemViewProps = {
  item: SidebarItem
  selected: boolean
  renderPin: boolean
  resourceView: ResourceView
  pathBuilder: PathBuilder
}

function buildStatusText(item: SidebarItem): string {
  let buildDur = item.lastBuildDur ? formatBuildDuration(item.lastBuildDur) : ""
  let buildStatus = item.buildStatus
  if (buildStatus === ResourceStatus.Pending) {
    return "Pending"
  } else if (buildStatus === ResourceStatus.Building) {
    return "Updating…"
  } else if (buildStatus === ResourceStatus.Unhealthy) {
    return "Update error"
  } else if (buildStatus === ResourceStatus.Healthy) {
    let msg = `Completed in ${buildDur}`
    if (item.buildAlertCount > 0) {
      msg += ", with issues"
    }
    return msg
  }
  return "Unknown"
}

export function SidebarItemView(props: SidebarItemViewProps) {
  let item = props.item
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
  let timeAgo = <TimeAgo date={item.lastDeployTime} formatter={formatter} />
  let isSelected = props.selected

  let isSelectedClass = isSelected ? "isSelected" : ""
  let isBuildingClass = building ? "isBuilding" : ""
  let onTrigger = triggerUpdate.bind(null, item.name)
  let renderPin = props.renderPin

  return (
    <SidebarItemRoot
      key={item.name}
      className={`${isSelectedClass} ${isBuildingClass}`}
    >
      {renderPin ? (
        <SidebarPinButton resourceName={item.name} />
      ) : (
        <SidebarPinButtonSpacer />
      )}
      <SidebarItemBox
        className={`${isSelectedClass} ${isBuildingClass}`}
        to={props.pathBuilder.path(link)}
        title={item.name}
      >
        <SidebarItemRuntimeBox>
          <SidebarIcon
            status={item.runtimeStatus}
            alertCount={item.runtimeAlertCount}
          />
          <SidebarItemName>{item.name}</SidebarItemName>
          <SidebarItemTimeAgo>
            {hasSuccessfullyDeployed ? timeAgo : "—"}
          </SidebarItemTimeAgo>
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
        </SidebarItemRuntimeBox>
        <SidebarItemBuildBox>
          <SidebarIcon
            status={item.buildStatus}
            alertCount={item.buildAlertCount}
          />
          <SidebarItemText>{buildStatusText(item)}</SidebarItemText>
        </SidebarItemBuildBox>
      </SidebarItemBox>
    </SidebarItemRoot>
  )
}

function PinnedItems(props: SidebarProps) {
  let ctx = useContext(sidebarPinContext)
  let pinnedItems = ctx.pinnedResources?.flatMap((r) =>
    props.items
      .filter((i) => i.name === r)
      .map((i) =>
        SidebarItemView({
          item: i,
          selected: props.selected === i.name,
          renderPin: false,
          pathBuilder: props.pathBuilder,
          resourceView: props.resourceView,
        })
      )
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
      .map((i) => i.buildAlertCount + i.runtimeAlertCount)
      .reduce((sum, current) => sum + current, 0)

    let listItems = this.props.items.map((item) =>
      SidebarItemView({
        item: item,
        selected: this.props.selected === item.name,
        renderPin: true,
        pathBuilder: this.props.pathBuilder,
        resourceView: this.props.resourceView,
      })
    )

    let nothingSelected = !this.props.selected

    return (
      <SidebarResourcesRoot className="Sidebar-resources">
        <SidebarList>
          <SidebarListSection name="">
            <SidebarItemAll
              nothingSelected={nothingSelected}
              allLink={allLink}
              totalAlerts={totalAlerts}
            />
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
