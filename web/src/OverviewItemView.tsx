import Collapse from "@material-ui/core/Collapse"
import Popover from "@material-ui/core/Popover"
import { makeStyles } from "@material-ui/core/styles"
import React, { useEffect, useRef, useState } from "react"
import { Link } from "react-router-dom"
import TimeAgo from "react-timeago"
import styled, { css, keyframes } from "styled-components"
import { buildAlerts, runtimeAlerts } from "./alerts"
import { incr } from "./analytics"
import { ReactComponent as CheckmarkSvg } from "./assets/svg/checkmark.svg"
import { ReactComponent as CopySvg } from "./assets/svg/copy.svg"
import { ReactComponent as LinkSvg } from "./assets/svg/link.svg"
import { ReactComponent as MaximizeSvg } from "./assets/svg/maximize.svg"
import { InstrumentedButton } from "./instrumentedComponents"
import { displayURL } from "./links"
import { usePathBuilder } from "./PathBuilder"
import SidebarIcon from "./SidebarIcon"
import SidebarTriggerButton from "./SidebarTriggerButton"
import StarResourceButton, {
  StarResourceButtonRoot,
} from "./StarResourceButton"
import { buildStatus, runtimeStatus } from "./status"
import {
  AnimDuration,
  Color,
  ColorAlpha,
  ColorRGBA,
  Font,
  FontSize,
  mixinTruncateText,
  overviewItemBorderRadius,
  SizeUnit,
  Width,
} from "./style-helpers"
import { formatBuildDuration, isZeroTime, timeDiff } from "./time"
import { timeAgoFormatter } from "./timeFormatters"
import { TriggerModeToggle } from "./TriggerModeToggle"
import { ResourceStatus, TargetType, TriggerMode } from "./types"

export const OverviewItemRoot = styled.li`
  display: flex;
  min-width: 330px;
  width: calc((100% - 3 * ${SizeUnit(0.75)} - 2 * ${SizeUnit(1)}) / 4);
  box-sizing: border-box;
  margin: 0 0 ${SizeUnit(0.75)} ${SizeUnit(0.75)};
`

type Resource = Proto.webviewResource
type Build = Proto.webviewBuildRecord

function resourceTypeLabel(res: Resource): string {
  if (res.isTiltfile) {
    return "Tiltfile"
  }
  let specs = res.specs ?? []
  for (let i = 0; i < specs.length; i++) {
    let spec = specs[i]
    if (spec.type === TargetType.K8s) {
      return "Kubernetes Deploy"
    } else if (spec.type === TargetType.DockerCompose) {
      return "Docker Compose Service"
    } else if (spec.type === TargetType.Local) {
      if (res.localResourceInfo && !!res.localResourceInfo.isTest) {
        return "Test"
      }
      return "Local Script"
    }
  }
  return "Unknown"
}

export class OverviewItem {
  name: string
  resourceTypeLabel: string
  isTiltfile: boolean
  isTest: boolean
  buildStatus: ResourceStatus
  buildAlertCount: number
  runtimeStatus: ResourceStatus
  runtimeAlertCount: number
  hasEndpoints: boolean
  lastBuildDur: moment.Duration | null
  lastDeployTime: string
  pendingBuildSince: string
  currentBuildStartTime: string
  triggerMode: TriggerMode
  hasPendingChanges: boolean
  queued: boolean
  lastBuild: Build | null = null
  endpoints: Proto.webviewLink[]
  podId: string

  /**
   * Create a pared down OverviewItem from a ResourceView
   */
  constructor(res: Resource) {
    let buildHistory = res.buildHistory || []
    let lastBuild = buildHistory.length > 0 ? buildHistory[0] : null

    this.name = res.name ?? ""
    this.isTiltfile = !!res.isTiltfile
    this.isTest =
      (res.localResourceInfo && !!res.localResourceInfo.isTest) || false
    this.buildStatus = buildStatus(res)
    this.buildAlertCount = buildAlerts(res, null).length
    this.runtimeStatus = runtimeStatus(res)
    this.runtimeAlertCount = runtimeAlerts(res, null).length
    this.hasEndpoints = (res.endpointLinks || []).length > 0
    this.lastBuildDur =
      lastBuild && lastBuild.startTime && lastBuild.finishTime
        ? timeDiff(lastBuild.startTime, lastBuild.finishTime)
        : null
    this.lastDeployTime = res.lastDeployTime ?? ""
    this.pendingBuildSince = res.pendingBuildSince ?? ""
    this.currentBuildStartTime = res.currentBuild?.startTime ?? ""
    this.triggerMode = res.triggerMode ?? TriggerMode.TriggerModeAuto
    this.hasPendingChanges = !!res.hasPendingChanges
    this.queued = !!res.queued
    this.lastBuild = lastBuild
    this.resourceTypeLabel = resourceTypeLabel(res)
    this.endpoints = res.endpointLinks ?? []
    this.podId = res.podID ?? ""
  }
}

const barberpole = keyframes`
  100% {
    background-position: 100% 100%;
  }
`

// `OverviewItemBox` is a flexbox column with two children:
//
// `OverviewItemRuntimeBox` has the runtime status, resource type, name, trigger button
//   +----+   +------------------------+
//   |Sta-+   | InnerRuntimeBox        |
//   |tus |   +------------------------+ <- Inside RuntimeBoxStack
//   |Icon|   | InnerRuntimeBox        |
//   +----+   +------------------------+
//
// `OverviewItemBuildBox`, right below, has build status, additional info, and trigger mode
//   +----+   +-------------------+  +-----------+
//   |Icon|   | OverviewItemText  |  |TriggerMode|
//   +----+   +-------------------+  +-----------+

export let OverviewItemBox = styled.div`
  color: ${Color.white};
  background-color: ${Color.gray};
  display: flex;
  flex-grow: 1;
  flex-direction: column;
  transition: color ${AnimDuration.default} linear,
    background-color ${AnimDuration.default} linear;
  overflow: hidden;
  border: 1px solid ${Color.grayLighter};
  position: relative; // Anchor .isBuilding::after + OverviewItemActions
  font-size: ${FontSize.small};
  font-family: ${Font.monospace};
  box-shadow: 0px 3px 3px 0px rgba(0, 0, 0, ${ColorAlpha.translucent});
  border-radius: ${overviewItemBorderRadius};
  padding: 0;

  &:hover {
    background-color: ${ColorRGBA(Color.gray, ColorAlpha.translucent)};
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

let OverviewItemRuntimeBox = styled.div`
  display: flex;
  align-items: stretch;
  transition: border-color ${AnimDuration.default} linear;
  flex-grow: 1;
`
let RuntimeBoxStack = styled.div`
  display: flex;
  flex-direction: column;
  flex-grow: 1;
  flex-shrink: 1;
  // To truncate long resource names…
  min-width: 0; // Override default, so width can be less than content
`
let InnerRuntimeBox = styled.div`
  display: flex;
  align-items: center;
  flex-grow: 1;

  ${StarResourceButtonRoot} {
    margin-left: ${SizeUnit(0.125)};
    align-content: center;
  }
`
let OverviewItemType = styled.div`
  display: flex;
  align-items: center;
  color: ${Color.grayLightest};
  opacity: ${ColorAlpha.almostOpaque};
`
let OverviewItemNameRoot = styled.div`
  display: flex;
  font-family: ${Font.sansSerif};
  font-weight: 600;
  padding-bottom: 8px;
  z-index: 1; // Appear above the .isBuilding gradient
  // To truncate long resource names…
  min-width: 0; // Override default, so width can be less than content
`
let OverviewItemNameTruncate = styled.span`
  ${mixinTruncateText}
`

let OverviewItemName = (props: { name: string }) => {
  // A common complaint is that long names get truncated, so we
  // use a title prop so that the user can see the full name.
  return (
    <OverviewItemNameRoot title={props.name}>
      <OverviewItemNameTruncate>{props.name}</OverviewItemNameTruncate>
    </OverviewItemNameRoot>
  )
}

let OverviewItemTimeAgo = styled.span`
  opacity: ${ColorAlpha.almostOpaque};
  margin-right: ${SizeUnit(0.5)};
  flex-grow: 1;
  text-align: right;
`

let OverviewItemBuildBox = styled.div`
  display: flex;
  align-items: stretch;
  flex-shrink: 1;
  border-top: 1px solid ${Color.grayLighter};
  padding-right: 4px;
`

let OverviewItemBuildText = styled.div`
  color: ${Color.grayLightest};
  display: flex;
  align-items: center;
  flex-grow: 1;
  padding-top: 4px;
  padding-bottom: 4px;
  ${mixinTruncateText}
`

export function triggerUpdate(name: string) {
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

export function toggleTriggerMode(name: string, mode: TriggerMode) {
  let url = "/api/override/trigger_mode"

  fetch(url, {
    method: "post",
    body: JSON.stringify({
      manifest_names: [name],
      trigger_mode: mode,
    }),
  }).then((response) => {
    if (!response.ok) {
      console.log(response)
    }
  })
}

export type OverviewItemViewProps = {
  item: OverviewItem
}

function buildStatusText(item: OverviewItem): string {
  let buildDur = item.lastBuildDur ? formatBuildDuration(item.lastBuildDur) : ""
  let buildStatus = item.buildStatus
  if (buildStatus === ResourceStatus.Pending) {
    return "Pending"
  } else if (buildStatus === ResourceStatus.Building) {
    return "Updating…"
  } else if (buildStatus === ResourceStatus.None) {
    return "No update status"
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

function buildTooltipText(status: ResourceStatus): string {
  switch (status) {
    case ResourceStatus.Building:
      return "Update: in progress"
    case ResourceStatus.Pending:
      return "Update: pending"
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

type RuntimeBoxProps = {
  item: OverviewItem
}

function RuntimeBox(props: RuntimeBoxProps) {
  let { item } = props

  let formatter = timeAgoFormatter
  let timeAgo = <TimeAgo date={item.lastDeployTime} formatter={formatter} />

  let building = !isZeroTime(item.currentBuildStartTime)
  let hasSuccessfullyDeployed = !isZeroTime(item.lastDeployTime)
  let hasBuilt = item.lastBuild !== null
  let onTrigger = triggerUpdate.bind(null, item.name)

  return (
    <OverviewItemRuntimeBox>
      <SidebarIcon
        tooltipText={runtimeTooltipText(item.runtimeStatus)}
        status={item.runtimeStatus}
        alertCount={item.runtimeAlertCount}
      />
      <RuntimeBoxStack>
        <InnerRuntimeBox>
          <OverviewItemType>{item.resourceTypeLabel}</OverviewItemType>
          <StarResourceButton
            resourceName={item.name}
            analyticsName="ui.web.overviewStarButton"
          />
          <OverviewItemTimeAgo>
            {hasSuccessfullyDeployed ? timeAgo : "—"}
          </OverviewItemTimeAgo>
          <SidebarTriggerButton
            isTiltfile={item.isTiltfile}
            isSelected={false}
            hasPendingChanges={item.hasPendingChanges}
            hasBuilt={hasBuilt}
            isBuilding={building}
            triggerMode={item.triggerMode}
            isQueued={item.queued}
            onTrigger={onTrigger}
          />
        </InnerRuntimeBox>
        <InnerRuntimeBox>
          <OverviewItemName name={item.name} />
        </InnerRuntimeBox>
      </RuntimeBoxStack>
    </OverviewItemRuntimeBox>
  )
}

type BuildBoxProps = {
  item: OverviewItem
  isDetailsBox: boolean
}

function BuildBox(props: BuildBoxProps) {
  let { item } = props
  let onModeToggle = toggleTriggerMode.bind(null, item.name)

  return (
    <OverviewItemBuildBox>
      <SidebarIcon
        tooltipText={buildTooltipText(item.buildStatus)}
        status={item.buildStatus}
        alertCount={item.buildAlertCount}
      />
      <OverviewItemBuildText>{buildStatusText(item)}</OverviewItemBuildText>
      {item.isTest && (
        <TriggerModeToggle
          triggerMode={item.triggerMode}
          onModeToggle={onModeToggle}
        />
      )}
    </OverviewItemBuildBox>
  )
}

type OverviewItemDetailsProps = {
  item: OverviewItem
  width: number
  height: number
}

let OverviewItemDetailsRoot = styled.div`
  display: flex;
  min-width: 330px;
  box-sizing: border-box;
  transition: height 200ms ease;

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

let OverviewItemDetailsBox = styled.div`
  color: ${Color.gray7};
  background-color: ${Color.gray};
  display: flex;
  flex-direction: column;
  overflow: hidden;
  width: 100%;
  border: 1px solid ${Color.grayLighter};
  position: relative;
  font-size: ${FontSize.small};
  font-family: ${Font.monospace};
  box-shadow: 0px 3px 3px 0px rgba(0, 0, 0, ${ColorAlpha.almostOpaque});
  border-radius: ${overviewItemBorderRadius};
`

let OverviewItemDetailsLinkBox = styled.div`
  margin-left: ${Width.statusIcon - 1}px;
  border-left: 1px solid ${Color.grayLighter};
  display: flex;
  flex-direction: column;
  width: 100%;
`

let detailsRow = css`
  outline: none !important;
  display: flex;
  background: transparent;
  cursor: pointer;
  padding: 0;
  border: 0;
  align-items: center;
  text-decoration: none;
  font: inherit;
  color: inherit;
  margin: 8px 0 8px ${Width.statusIconMarginRight}px;
  transition: color ${AnimDuration.default} ease;
  padding-right: ${Width.statusIcon}px;

  & .fillStd {
    fill: ${Color.gray7};
    transition: fill ${AnimDuration.default} ease;
  }
  &:hover {
    color: ${Color.blue};
  }
  &:hover .fillStd {
    fill: ${Color.blue};
  }
`

let Endpoint = styled.a`
  ${detailsRow}
`

let Copy = styled(InstrumentedButton)`
  ${detailsRow}
`

let ShowDetailsBox = styled(Link)`
  ${detailsRow}
`

let DetailText = styled.div`
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-left: 10px;
`

let DetailTextBold = styled(DetailText)`
  font-weight: 700;
`

async function copyTextToClipboard(text: string, cb: () => void) {
  await navigator.clipboard.writeText(text)
  cb()
}

// OverviewItemDetails is a clone of the OverviewItemView, positioned
// with a popover over the original OverviewItemDetails
export function OverviewItemDetails(props: OverviewItemDetailsProps) {
  let { item, width, height } = props
  let pathBuilder = usePathBuilder()
  let link = pathBuilder.encpath`/r/${item.name}/overview`
  let endpoints = item.endpoints.map((ep) => {
    return (
      <Endpoint
        onClick={() => void incr("ui.web.endpoint", { action: "click" })}
        href={ep.url}
        // We use ep.url as the target, so that clicking the link re-uses the tab.
        target={ep.url}
        key={ep.url}
      >
        <LinkSvg />
        <DetailText>{ep.name || displayURL(ep)}</DetailText>
      </Endpoint>
    )
  })

  let copy: React.ReactElement | null = null
  let [showCopySuccess, setShowCopySuccess] = useState(false)

  if (item.podId) {
    let copyClick = () => {
      copyTextToClipboard(item.podId, () => {
        setShowCopySuccess(true)

        setTimeout(() => {
          setShowCopySuccess(false)
        }, 5000)
      })
    }

    let icon = showCopySuccess ? (
      <CheckmarkSvg width="20" height="20" />
    ) : (
      <CopySvg width="20" height="20" />
    )

    copy = (
      <Copy onClick={copyClick} analyticsName="ui.web.overview.copyPodID">
        {icon}
        <DetailText style={{ marginLeft: "8px" }}>
          {item.podId} Pod ID
        </DetailText>
      </Copy>
    )
  }

  let isBuildingClass =
    item.buildStatus === ResourceStatus.Building ? "isBuilding" : ""
  let ref = useRef(null as any)

  // Simulate an 'expansion' animation by modifying the height post-render.
  useEffect(() => {
    if (ref.current && height) {
      ref.current.style.height = ref.current.firstChild.scrollHeight + "px"
    }
  }, [height])

  let widthStyle = width ? width + "px" : "auto"
  let heightStyle = height ? height + "px" : "auto"
  return (
    <OverviewItemDetailsRoot
      ref={ref}
      style={{ width: widthStyle, height: heightStyle }}
      className={isBuildingClass}
    >
      <OverviewItemDetailsBox>
        <RuntimeBox item={item} />
        <OverviewItemDetailsLinkBox>
          {endpoints}
          {copy}
          <ShowDetailsBox to={link}>
            <MaximizeSvg />

            <DetailTextBold>Show details</DetailTextBold>
          </ShowDetailsBox>
        </OverviewItemDetailsLinkBox>
        <BuildBox item={props.item} isDetailsBox={true} />
      </OverviewItemDetailsBox>
    </OverviewItemDetailsRoot>
  )
}

let useStyles = makeStyles((theme) => ({
  paper: {
    background: "transparent",
    boxShadow: "none",
    borderRadius: "0",
    overflow: "visible",
  },
}))

export default function OverviewItemView(props: OverviewItemViewProps) {
  const popoverClasses = useStyles()

  let [anchorSpec, setAnchorSpec] = useState({
    element: null as Element | null,
    width: 330,
    height: 0,
  })
  let handleClick = (event: any) => {
    let currentTarget = event.currentTarget
    let rect = currentTarget.getBoundingClientRect()
    setAnchorSpec({
      element: currentTarget,
      width: rect.width,
      height: rect.height,
    })
  }
  let handleClose = (e: any) => {
    e.stopPropagation()
    setAnchorSpec({
      element: null,
      width: anchorSpec.width,
      height: anchorSpec.height,
    })
  }

  let open = Boolean(anchorSpec.element)
  let popoverId = open ? "item-open-popover" : undefined

  let item = props.item
  let building = item.buildStatus === ResourceStatus.Building
  let isBuildingClass = building ? "isBuilding" : ""

  return (
    <OverviewItemRoot
      key={item.name}
      onClick={handleClick}
      className="u-showStarOnHover u-showTriggerModeOnHover"
    >
      <OverviewItemBox className={`${isBuildingClass}`} data-name={item.name}>
        <RuntimeBox item={item} />
        <BuildBox item={item} isDetailsBox={false} />
      </OverviewItemBox>

      <Popover
        id={popoverId}
        classes={popoverClasses}
        open={open}
        anchorEl={anchorSpec.element}
        onClose={handleClose}
        disableScrollLock={true}
        TransitionComponent={Collapse}
        anchorOrigin={{
          vertical: "top",
          horizontal: "center",
        }}
        transformOrigin={{
          vertical: "top",
          horizontal: "center",
        }}
      >
        <OverviewItemDetails
          item={item}
          width={anchorSpec.width}
          height={anchorSpec.height}
        />
      </Popover>
    </OverviewItemRoot>
  )
}
