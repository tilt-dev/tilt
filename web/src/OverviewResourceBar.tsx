import _ from "lodash"
import React, { Component, useRef, useState } from "react"
import { Link } from "react-router-dom"
import styled from "styled-components"
import { AccountMenuContent, AccountMenuHeader } from "./AccountMenu"
import { incr } from "./analytics"
import { ReactComponent as AccountIcon } from "./assets/svg/account.svg"
import { ReactComponent as CheckmarkSmallSvg } from "./assets/svg/checkmark-small.svg"
import { ReactComponent as CloseSvg } from "./assets/svg/close.svg"
import { ReactComponent as HelpIcon } from "./assets/svg/help.svg"
import { ReactComponent as MetricsIcon } from "./assets/svg/metrics.svg"
import { ReactComponent as SnapshotIcon } from "./assets/svg/snapshot.svg"
import { ReactComponent as UpdateAvailableIcon } from "./assets/svg/update-available.svg"
import { ReactComponent as WarningSvg } from "./assets/svg/warning.svg"
import FloatDialog from "./FloatDialog"
import { FilterLevel } from "./logfilters"
import MetricsDialog from "./MetricsDialog"
import { usePathBuilder } from "./PathBuilder"
import ShortcutsDialog from "./ShortcutsDialog"
import { SnapshotAction, useSnapshotAction } from "./snapshot"
import { combinedStatus } from "./status"
import {
  AnimDuration,
  Color,
  Font,
  FontSize,
  mixinResetButtonStyle,
  mixinResetListStyle,
  SizeUnit,
} from "./style-helpers"
import { ResourceName, ResourceStatus, TargetType } from "./types"
import UpdateDialog, { showUpdate } from "./UpdateDialog"

type MetricsServing = Proto.webviewMetricsServing

type OverviewResourceBarProps = {
  view: Proto.webviewView
}

let OverviewResourceBarRoot = styled.div`
  display: flex;
  align-items: stretch;
  padding-left: ${SizeUnit(1)};
`

let ResourceBarEndRoot = styled.div`
  flex-grow: 1;
  display: flex;
  align-items: stretch;
  justify-content: flex-end;
`

let ResourceBarStatusRoot = styled.div`
  display: flex;
  font-family: ${Font.sansSerif};
  font-size: ${FontSize.smallester};
  align-items: center;
  color: ${Color.grayLightest};

  .fillStd {
    fill: ${Color.grayLighter};
  }

  & + & {
    margin-left: ${SizeUnit(1.5)};
  }
`

let ResourceBarStatusLabel = styled.p`
  text-transform: uppercase;
  margin-right: ${SizeUnit(0.5)};
`

let ResourceBarStatusSummaryList = styled.ul`
  display: flex;
  ${mixinResetListStyle}
`
let ResourceBarStatusSummaryItem = styled.li`
  display: flex;
  align-items: center;

  & + & {
    margin-left: ${SizeUnit(0.25)};
    border-left: 1px solid ${Color.grayLighter};
    padding-left: ${SizeUnit(0.25)};
  }
  &.hasErr {
    color: ${Color.red};
    .fillStd {
      fill: ${Color.red};
    }
  }
  &.hasWarn {
    color: ${Color.yellow};
    .fillStd {
      fill: ${Color.yellow};
    }
  }
  &.hasTotalHealthy {
    color: ${Color.green};
    .fillStd {
      fill: ${Color.green};
    }
  }
`

let ResourceBarStatusSummaryItemCount = styled.span`
  font-weight: strong;
  padding-left: 4px;
  padding-right: 4px;
`
type ResourceBarStatusProps = {
  view: Proto.webviewView
}

type ResourceGroupStatusProps = {
  counts: StatusCounts
  label: string
  healthyLabel: string
  unhealthyLabel: string
  warningLabel: string
}

function ResourceGroupStatus(props: ResourceGroupStatusProps) {
  if (props.counts.total === 0) {
    return null
  }
  let pb = usePathBuilder()

  let errorLink = pb.path(
    `/r/${ResourceName.all}/overview?level=${FilterLevel.error}`
  )
  let warnLink = pb.path(
    `/r/${ResourceName.all}/overview?level=${FilterLevel.warn}`
  )

  return (
    <ResourceBarStatusRoot>
      <ResourceBarStatusLabel>{props.label}</ResourceBarStatusLabel>
      <ResourceBarStatusSummaryList>
        <ResourceBarStatusSummaryItem
          className={props.counts.unhealthy >= 1 ? "hasErr" : ""}
        >
          <CloseSvg width="11" />
          <Link to={errorLink}>
            <ResourceBarStatusSummaryItemCount>
              {props.counts.unhealthy}
            </ResourceBarStatusSummaryItemCount>{" "}
            {props.unhealthyLabel}
          </Link>
        </ResourceBarStatusSummaryItem>
        <ResourceBarStatusSummaryItem
          className={props.counts.warning >= 1 ? "hasWarn" : ""}
        >
          <WarningSvg width="7" />
          <Link to={warnLink}>
            <ResourceBarStatusSummaryItemCount>
              {props.counts.warning}
            </ResourceBarStatusSummaryItemCount>{" "}
            {props.warningLabel}
          </Link>
        </ResourceBarStatusSummaryItem>
        <ResourceBarStatusSummaryItem
          className={
            props.counts.healthy === props.counts.total ? "hasTotalHealthy" : ""
          }
        >
          <CheckmarkSmallSvg />
          <ResourceBarStatusSummaryItemCount>
            {props.counts.healthy}
          </ResourceBarStatusSummaryItemCount>
          /
          <ResourceBarStatusSummaryItemCount>
            {props.counts.total}
          </ResourceBarStatusSummaryItemCount>{" "}
          {props.healthyLabel}
        </ResourceBarStatusSummaryItem>
      </ResourceBarStatusSummaryList>
    </ResourceBarStatusRoot>
  )
}

type StatusCounts = {
  total: number
  healthy: number
  unhealthy: number
  pending: number
  warning: number
}

function statusCounts(resources: Proto.webviewResource[]): StatusCounts {
  let statuses = resources.map((res) => combinedStatus(res))
  let allStatusCount = 0
  let healthyStatusCount = 0
  let unhealthyStatusCount = 0
  let pendingStatusCount = 0
  let warningCount = 0
  statuses.forEach((status) => {
    switch (status) {
      case ResourceStatus.Warning:
        allStatusCount++
        healthyStatusCount++
        warningCount++
        break
      case ResourceStatus.Healthy:
        allStatusCount++
        healthyStatusCount++
        break
      case ResourceStatus.Unhealthy:
        allStatusCount++
        unhealthyStatusCount++
        break
      case ResourceStatus.Pending:
      case ResourceStatus.Building:
        allStatusCount++
        pendingStatusCount++
        break
      default:
      // Don't count None status in the overall resource count.
      // These might be manual tasks we haven't run yet.
    }
  })

  return {
    total: allStatusCount,
    healthy: healthyStatusCount,
    unhealthy: unhealthyStatusCount,
    pending: pendingStatusCount,
    warning: warningCount,
  }
}

function ResourceBarStatus(props: ResourceBarStatusProps) {
  // Count the statuses.
  let resources = props.view.resources || []

  let [testResources, otherResources] = _.partition<Proto.webviewResource>(
    resources,
    (r) => r.localResourceInfo && r.localResourceInfo.isTest
  )

  return (
    <>
      <ResourceGroupStatus
        counts={statusCounts(otherResources)}
        label={"Resources"}
        healthyLabel={"healthy"}
        unhealthyLabel={"err"}
        warningLabel={"warn"}
      />
      <ResourceGroupStatus
        counts={statusCounts(testResources)}
        label={"Tests"}
        healthyLabel={"pass"}
        unhealthyLabel={"fail"}
        warningLabel={"warn"}
      />
    </>
  )
}

export let MenuButton = styled.button`
  ${mixinResetButtonStyle}
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  align-items: center;
  transition: color ${AnimDuration.default} ease;
  position: relative; // Anchor MenuButtonLabel, which shouldn't affect this element's width
  padding-top: ${SizeUnit(0.5)};
  padding-left: ${SizeUnit(0.5)};
  padding-right: ${SizeUnit(0.5)};
  padding-bottom: ${SizeUnit(0.5)};
  font-size: ${FontSize.smallest};
  color: ${Color.blue};

  & .fillStd {
    fill: ${Color.blue};
    transition: fill ${AnimDuration.default} ease;
  }
  &:hover .fillStd {
    fill: ${Color.blueLight};
  }
  & .fillBg {
    fill: ${Color.grayDarker};
  }

  &.is-disabled {
    pointer-events: none;
    cursor: default;
  }
`

let MenuButtonLabel = styled.div`
  position: absolute;
  bottom: 0;
  font-size: ${FontSize.smallester};
  color: ${Color.blueDark};
  width: 200%;
  transition: opacity ${AnimDuration.default} ease;
  opacity: 0;

  ${MenuButton}:hover &,
  ${MenuButton}[data-open="true"] & {
    opacity: 1;
  }
`

let UpdateAvailableFloatIcon = styled(UpdateAvailableIcon)`
  display: block;
  position: absolute;
  top: 8px;
  left: 5px;
  width: 10px;
  height: 10px;
`

type ResourceBarShortcutsProps = {
  toggleShortcutsDialog: () => void
  snapshot: SnapshotAction
}

/**
 * Sets up keyboard shortcuts that depend on the resource bar block.
 */
class ResourceBarShortcuts extends Component<ResourceBarShortcutsProps> {
  constructor(props: ResourceBarShortcutsProps) {
    super(props)
    this.onKeydown = this.onKeydown.bind(this)
  }

  componentDidMount() {
    document.body.addEventListener("keydown", this.onKeydown)
  }

  componentWillUnmount() {
    document.body.removeEventListener("keydown", this.onKeydown)
  }

  onKeydown(e: KeyboardEvent) {
    if (e.metaKey || e.altKey || e.ctrlKey || e.isComposing) {
      return
    }
    if (e.key === "?") {
      this.props.toggleShortcutsDialog()
      e.preventDefault()
    } else if (e.key === "s" && this.props.snapshot.enabled) {
      this.props.snapshot.openModal()
      e.preventDefault()
    }
  }

  render() {
    return <span></span>
  }
}

type ResourceBarEndProps = {
  isSnapshot: boolean
  tiltCloudUsername: string
  tiltCloudSchemeHost: string
  tiltCloudTeamID: string
  tiltCloudTeamName: string
  snapshot: SnapshotAction
  showUpdate: boolean
  suggestedVersion: string | null | undefined
  runningBuild: Proto.webviewTiltBuild | undefined
  showMetricsButton: boolean
  metricsServing: MetricsServing | null | undefined
}

function ResourceBarEnd(props: ResourceBarEndProps) {
  const shortcutButton = useRef(null as any)
  const accountButton = useRef(null as any)
  const updateButton = useRef(null as any)
  const metricsButton = useRef(null as any)
  const [shortcutsDialogAnchor, setShortcutsDialogAnchor] = useState(
    null as Element | null
  )
  const [accountMenuAnchor, setAccountMenuAnchor] = useState(
    null as Element | null
  )
  const [updateDialogAnchor, setUpdateDialogAnchor] = useState(
    null as Element | null
  )
  const [metricsDialogAnchor, setMetricsDialogAnchor] = useState(
    null as Element | null
  )
  const shortcutsDialogOpen = !!shortcutsDialogAnchor
  const accountMenuOpen = !!accountMenuAnchor
  const updateDialogOpen = !!updateDialogAnchor
  const metricsDialogOpen = !!metricsDialogAnchor
  let isSnapshot = props.isSnapshot
  if (isSnapshot) {
    return null
  }

  let toggleAccountMenu = (action: string) => {
    if (!accountMenuOpen) {
      incr("ui.web.menu", { type: "account", action: action })
    }
    setAccountMenuAnchor(
      accountMenuOpen ? null : (accountButton.current as Element)
    )
  }

  let toggleShortcutsDialog = (action: string) => {
    if (!shortcutsDialogOpen) {
      incr("ui.web.menu", { type: "shortcuts", action: action })
    }
    setShortcutsDialogAnchor(
      shortcutsDialogOpen ? null : (shortcutButton.current as Element)
    )
  }

  let toggleUpdateDialog = (action: string) => {
    if (!updateDialogOpen) {
      incr("ui.web.menu", { type: "update", action: action })
    }
    setUpdateDialogAnchor(
      updateDialogOpen ? null : (updateButton.current as Element)
    )
  }

  let toggleMetricsDialog = (action: string) => {
    if (!metricsDialogOpen) {
      incr("ui.web.menu", { type: "metrics", action: action })
    }
    setMetricsDialogAnchor(
      metricsDialogOpen ? null : (metricsButton.current as Element)
    )
  }

  let accountMenuHeader = <AccountMenuHeader {...props} />
  let accountMenuContent = <AccountMenuContent {...props} />
  let snapshotButton = props.snapshot.enabled ? (
    <MenuButton onClick={props.snapshot.openModal}>
      <SnapshotIcon width="24" height="24" />
      <MenuButtonLabel>Make Snapshot</MenuButtonLabel>
    </MenuButton>
  ) : null

  let metricsButtonEl = props.showMetricsButton ? (
    <MenuButton
      ref={metricsButton}
      onClick={() => toggleMetricsDialog("click")}
      data-open={metricsDialogOpen}
    >
      <MetricsIcon width="24" height="24" />
      <MenuButtonLabel>Metrics</MenuButtonLabel>
    </MenuButton>
  ) : null

  return (
    <ResourceBarEndRoot>
      <MenuButton
        ref={updateButton}
        onClick={() => toggleUpdateDialog("click")}
        data-open={updateDialogOpen}
      >
        <div>v{props.runningBuild?.version || "?"}</div>

        {props.showUpdate ? <UpdateAvailableFloatIcon /> : null}
        <MenuButtonLabel>
          {props.showUpdate ? "Update Available" : "Tilt Version"}
        </MenuButtonLabel>
      </MenuButton>

      {snapshotButton}
      {metricsButtonEl}

      <MenuButton
        ref={shortcutButton}
        onClick={() => toggleShortcutsDialog("click")}
        data-open={shortcutsDialogOpen}
      >
        <HelpIcon width="24" height="24" />
        <MenuButtonLabel>Help</MenuButtonLabel>
      </MenuButton>
      <MenuButton
        ref={accountButton}
        onClick={() => toggleAccountMenu("click")}
        data-open={accountMenuOpen}
      >
        <AccountIcon width="24" height="24" />
        <MenuButtonLabel>Account</MenuButtonLabel>
      </MenuButton>

      <FloatDialog
        id="accountMenu"
        title={accountMenuHeader}
        open={accountMenuOpen}
        anchorEl={accountMenuAnchor}
        onClose={() => toggleAccountMenu("close")}
      >
        {accountMenuContent}
      </FloatDialog>
      <ShortcutsDialog
        open={shortcutsDialogOpen}
        anchorEl={shortcutsDialogAnchor}
        onClose={() => toggleShortcutsDialog("close")}
        isOverview={true}
      />
      <MetricsDialog
        open={metricsDialogOpen}
        anchorEl={metricsDialogAnchor}
        onClose={() => toggleMetricsDialog("close")}
        serving={props.metricsServing}
      />
      <UpdateDialog
        open={updateDialogOpen}
        anchorEl={updateDialogAnchor}
        onClose={() => toggleUpdateDialog("close")}
        showUpdate={props.showUpdate}
        suggestedVersion={props.suggestedVersion}
        isNewInterface={true}
      />
      <ResourceBarShortcuts
        toggleShortcutsDialog={() => toggleShortcutsDialog("shortcut")}
        snapshot={props.snapshot}
      />
    </ResourceBarEndRoot>
  )
}

export default function OverviewResourceBar(props: OverviewResourceBarProps) {
  let isSnapshot = usePathBuilder().isSnapshot()
  let snapshot = useSnapshotAction()
  let view = props.view
  let runningBuild = view?.runningTiltBuild
  let suggestedVersion = view?.suggestedTiltVersion
  let resources = view?.resources || []
  let hasK8s = resources.some((r) => {
    let specs = r.specs ?? []
    return specs.some((spec) => spec.type === TargetType.K8s)
  })
  let showMetricsButton = !!(hasK8s || view?.metricsServing?.mode)
  let metricsServing = view?.metricsServing

  let resourceBarEndProps = {
    isSnapshot,
    snapshot,
    showUpdate: showUpdate(view),
    suggestedVersion,
    runningBuild,
    showMetricsButton,
    metricsServing,
    tiltCloudUsername: view.tiltCloudUsername ?? "",
    tiltCloudSchemeHost: view.tiltCloudSchemeHost ?? "",
    tiltCloudTeamID: view.tiltCloudTeamID ?? "",
    tiltCloudTeamName: view.tiltCloudTeamName ?? "",
  }

  return (
    <OverviewResourceBarRoot>
      <ResourceBarStatus view={props.view} />
      <ResourceBarEnd {...resourceBarEndProps} />
    </OverviewResourceBarRoot>
  )
}
