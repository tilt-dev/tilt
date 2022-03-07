import React, { MutableRefObject, useEffect, useRef } from "react"
import TimeAgo from "react-timeago"
import styled from "styled-components"
import { Flag, useFeatures } from "./feature"
import { Hold } from "./Hold"
import PathBuilder from "./PathBuilder"
import { useResourceNav } from "./ResourceNav"
import { SidebarBuildButton } from "./SidebarBuildButton"
import SidebarIcon from "./SidebarIcon"
import SidebarItem from "./SidebarItem"
import StarResourceButton, {
  StarResourceButtonRoot,
} from "./StarResourceButton"
import { PendingBuildDescription } from "./status"
import {
  AnimDuration,
  barberpole,
  Color,
  ColorAlpha,
  ColorRGBA,
  Font,
  FontSize,
  mixinTruncateText,
  overviewItemBorderRadius,
  SizeUnit,
} from "./style-helpers"
import { formatBuildDuration, isZeroTime } from "./time"
import { timeAgoFormatter } from "./timeFormatters"
import { startBuild } from "./trigger"
import { ResourceStatus, ResourceView } from "./types"

export const SidebarItemRoot = styled.li`
  & + & {
    margin-top: ${SizeUnit(0.35)};
  }

  &.isDisabled + &.isDisabled {
    margin-top: ${SizeUnit(1 / 16)};
  }

  /* smaller margin-left since the star icon takes up space */
  margin-left: ${SizeUnit(0.25)};
  margin-right: ${SizeUnit(0.5)};
  display: flex;

  ${StarResourceButtonRoot} {
    margin-right: ${SizeUnit(1.0 / 12)};
  }

  /* groupViewIndent is used to indent un-grouped
     items so they align with grouped items */
  &.groupViewIndent {
    margin-left: ${SizeUnit(2 / 3)};
  }
`
// Shared styles between the enabled and disabled item boxes
const sidebarItemBoxMixin = `
  border-radius: ${overviewItemBorderRadius};
  cursor: pointer;
  display: flex;
  flex-grow: 1;
  font-size: ${FontSize.small};
  transition: color ${AnimDuration.default} linear,
              background-color ${AnimDuration.default} linear;
  overflow: hidden;
  text-decoration: none;
`

export let SidebarItemBox = styled.div`
  ${sidebarItemBoxMixin};
  background-color: ${Color.gray30};
  border: 1px solid ${Color.gray40};
  color: ${Color.white};
  font-family: ${Font.monospace};
  position: relative; /* Anchor the .isBuilding::after psuedo-element */

  &:hover {
    background-color: ${ColorRGBA(Color.gray30, ColorAlpha.translucent)};
  }

  &.isSelected {
    background-color: ${Color.white};
    color: ${Color.gray30};
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
      ${ColorRGBA(Color.gray50, ColorAlpha.translucent)},
      ${ColorRGBA(Color.gray50, ColorAlpha.translucent)} 1px,
      ${ColorRGBA(Color.black, 0)} 1px,
      ${ColorRGBA(Color.black, 0)} 6px
    );
    background-size: 200% 200%;
    animation: ${barberpole} 8s linear infinite;
    z-index: 0;
  }
`

const DisabledSidebarItemBox = styled.div`
  ${sidebarItemBoxMixin};
  color: ${Color.gray50};
  font-family: ${Font.sansSerif};
  font-style: italic;
  padding: ${SizeUnit(1 / 8)} ${SizeUnit(1 / 4)};

  &:hover {
    color: ${Color.blue};
  }

  &.isSelected {
    background-color: ${Color.gray70};
    color: ${Color.gray10};
    transition: color ${AnimDuration.default} linear,
      font-weight ${AnimDuration.default} linear;
    font-weight: normal;
  }
`

// Flexbox (column) containing:
// - `SidebarItemRuntimeBox` - (row) with runtime status, name, star, timeago
// - `SidebarItemBuildBox` - (row) with build status, text
let SidebarItemInnerBox = styled.div`
  display: flex;
  flex-direction: column;
  flex-grow: 1;
  // To truncate long resource names…
  min-width: 0; // Override default, so width can be less than content
`

let SidebarItemRuntimeBox = styled.div`
  display: flex;
  flex-grow: 1;
  align-items: stretch;
  height: ${SizeUnit(1)};
  border-bottom: 1px solid ${Color.gray40};
  box-sizing: border-box;
  transition: border-color ${AnimDuration.default} linear;

  .isSelected & {
    border-bottom-color: ${Color.grayLightest};
  }
`

let SidebarItemBuildBox = styled.div`
  display: flex;
  align-items: stretch;
  padding-right: 4px;
`
let SidebarItemText = styled.div`
  ${mixinTruncateText};
  align-items: center;
  flex-grow: 1;
  padding-top: 4px;
  padding-bottom: 4px;
  color: ${Color.grayLightest};
`

let SidebarItemNameRoot = styled.div`
  display: flex;
  align-items: center;
  font-family: ${Font.sansSerif};
  font-weight: 600;
  z-index: 1; // Appear above the .isBuilding gradient
  // To truncate long resource names…
  min-width: 0; // Override default, so width can be less than content
`
let SidebarItemNameTruncate = styled.span`
  ${mixinTruncateText}
`

export function sidebarItemIsDisabled(item: SidebarItem) {
  // Both build and runtime status are disabled when a resource
  // is disabled, so just reference runtime status here
  return item.runtimeStatus === ResourceStatus.Disabled
}

let SidebarItemName = (props: { name: string }) => {
  // A common complaint is that long names get truncated, so we
  // use a title prop so that the user can see the full name.
  return (
    <SidebarItemNameRoot title={props.name}>
      <SidebarItemNameTruncate>{props.name}</SidebarItemNameTruncate>
    </SidebarItemNameRoot>
  )
}

let SidebarItemTimeAgo = styled.span`
  opacity: ${ColorAlpha.almostOpaque};
  display: flex;
  justify-content: flex-end;
  flex-grow: 1;
  align-items: center;
  text-align: right;
  white-space: nowrap;
  padding-right: ${SizeUnit(0.25)};
`

export type SidebarItemViewProps = {
  item: SidebarItem
  selected: boolean
  resourceView: ResourceView
  pathBuilder: PathBuilder
  groupView?: boolean
}

function buildStatusText(item: SidebarItem): string {
  let buildDur = item.lastBuildDur ? formatBuildDuration(item.lastBuildDur) : ""
  let buildStatus = item.buildStatus
  if (buildStatus === ResourceStatus.Pending) {
    return holdStatusText(item.hold)
  } else if (buildStatus === ResourceStatus.Building) {
    return "Updating…"
  } else if (buildStatus === ResourceStatus.None) {
    return "No update status"
  } else if (buildStatus === ResourceStatus.Unhealthy) {
    return "Update error"
  } else if (buildStatus === ResourceStatus.Healthy) {
    return `Completed in ${buildDur}`
  } else if (buildStatus === ResourceStatus.Warning) {
    return `Completed in ${buildDur}, with issues`
  }
  return "Unknown"
}

function holdStatusText(hold?: Hold | null): string {
  if (!hold?.count) {
    return "Pending"
  }

  if (hold.clusters.length) {
    return "Waiting for cluster connection"
  }

  if (hold.images.length) {
    return "Waiting for shared image build"
  }

  if (hold.resources.length === 1) {
    // show the actual name
    return `Waiting on ${hold.resources[0]}`
  }

  let count: number
  let type: string
  if (hold.resources.length) {
    count = hold.resources.length
    type = "resources"
  } else {
    count = hold.count
    type = `object${hold.count > 1 ? "s" : ""}`
  }

  return `Waiting on ${count} ${type}`
}

function runtimeTooltipText(status: ResourceStatus): string {
  switch (status) {
    case ResourceStatus.Building:
      return "Server: deploying"
    case ResourceStatus.Pending:
      return "Server: pending"
    case ResourceStatus.Warning:
      return "Server: issues"
    case ResourceStatus.Healthy:
      return "Server: ready"
    case ResourceStatus.Unhealthy:
      return "Server: unhealthy"
    default:
      return "No server"
  }
}

function buildTooltipText(status: ResourceStatus, hold: Hold | null): string {
  switch (status) {
    case ResourceStatus.Building:
      return "Update: in progress"
    case ResourceStatus.Pending:
      return PendingBuildDescription(hold)
    case ResourceStatus.Warning:
      return "Update: warning"
    case ResourceStatus.Healthy:
      return "Update: success"
    case ResourceStatus.Unhealthy:
      return "Update: error"
    default:
      return "No update status"
  }
}

export function DisabledSidebarItemView(props: SidebarItemViewProps) {
  const { openResource } = useResourceNav()
  const { item, selected, groupView } = props
  const isSelectedClass = selected ? "isSelected" : ""
  const groupViewIndentClass = groupView ? "groupViewIndent" : ""
  let analyticsTags = { target: item.targetType }

  return (
    <SidebarItemRoot
      className={`u-showStarOnHover ${isSelectedClass} ${groupViewIndentClass} isDisabled`}
    >
      <StarResourceButton
        resourceName={item.name}
        analyticsName="ui.web.sidebarStarButton"
        analyticsTags={analyticsTags}
      />
      <DisabledSidebarItemBox
        className={`${isSelectedClass}`}
        onClick={(_e) => openResource(item.name)}
      >
        {item.name}
      </DisabledSidebarItemBox>
    </SidebarItemRoot>
  )
}

export function EnabledSidebarItemView(props: SidebarItemViewProps) {
  let nav = useResourceNav()
  let item = props.item
  let formatter = timeAgoFormatter
  let hasSuccessfullyDeployed = !isZeroTime(item.lastDeployTime)
  let hasBuilt = item.lastBuild !== null
  let building = !isZeroTime(item.currentBuildStartTime)
  let time = item.lastDeployTime || ""
  let timeAgo = <TimeAgo date={time} formatter={formatter} />
  let isSelected = props.selected

  let isSelectedClass = isSelected ? "isSelected" : ""
  let isBuildingClass = building ? "isBuilding" : ""
  let onStartBuild = startBuild.bind(null, item.name)
  const groupViewIndentClass = props.groupView ? "groupViewIndent" : ""
  let analyticsTags = { target: item.targetType }
  let ref: MutableRefObject<HTMLLIElement | null> = useRef(null)

  useEffect(() => {
    if (isSelected && ref.current?.scrollIntoView) {
      ref.current.scrollIntoView({ block: "nearest" })
    }
  }, [item.name, isSelected, ref])

  return (
    <SidebarItemRoot
      ref={ref}
      key={item.name}
      className={`u-showStarOnHover u-showTriggerModeOnHover ${isSelectedClass} ${isBuildingClass} ${groupViewIndentClass}`}
    >
      <StarResourceButton
        resourceName={item.name}
        analyticsName="ui.web.sidebarStarButton"
        analyticsTags={analyticsTags}
      />
      <SidebarItemBox
        className={`${isSelectedClass} ${isBuildingClass}`}
        tabIndex={-1}
        role="button"
        onClick={(e) => nav.openResource(item.name)}
        data-name={item.name}
      >
        <SidebarItemInnerBox>
          <SidebarItemRuntimeBox>
            <SidebarIcon
              tooltipText={runtimeTooltipText(item.runtimeStatus)}
              status={item.runtimeStatus}
              alertCount={item.runtimeAlertCount}
            />
            <SidebarItemName name={item.name} />
            <SidebarItemTimeAgo>
              {hasSuccessfullyDeployed ? timeAgo : "—"}
            </SidebarItemTimeAgo>
            <SidebarBuildButton
              isSelected={isSelected}
              hasPendingChanges={item.hasPendingChanges}
              hasBuilt={hasBuilt}
              isBuilding={building}
              triggerMode={item.triggerMode}
              isQueued={item.queued}
              onStartBuild={onStartBuild}
              analyticsTags={analyticsTags}
              stopBuildButton={item.stopBuildButton}
            />
          </SidebarItemRuntimeBox>
          <SidebarItemBuildBox>
            <SidebarIcon
              tooltipText={buildTooltipText(item.buildStatus, item.hold)}
              status={item.buildStatus}
              alertCount={item.buildAlertCount}
            />
            <SidebarItemText>{buildStatusText(item)}</SidebarItemText>
          </SidebarItemBuildBox>
        </SidebarItemInnerBox>
      </SidebarItemBox>
    </SidebarItemRoot>
  )
}

export default function SidebarItemView(props: SidebarItemViewProps) {
  const features = useFeatures()
  const showDisabledResources = features.isEnabled(Flag.DisableResources)
  const itemIsDisabled = sidebarItemIsDisabled(props.item)
  if (itemIsDisabled && !showDisabledResources) {
    return null
  } else if (itemIsDisabled && showDisabledResources) {
    return <DisabledSidebarItemView {...props}></DisabledSidebarItemView>
  } else {
    return <EnabledSidebarItemView {...props}></EnabledSidebarItemView>
  }
}
