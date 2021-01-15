import React, { Component, useRef, useState } from "react"
import styled from "styled-components"
import { AccountMenuContent, AccountMenuHeader } from "./AccountMenu"
import { incr } from "./analytics"
import { ReactComponent as AccountIcon } from "./assets/svg/account.svg"
import { ReactComponent as HelpIcon } from "./assets/svg/help.svg"
import { ReactComponent as SnapshotIcon } from "./assets/svg/snapshot.svg"
import { ReactComponent as UpdateAvailableIcon } from "./assets/svg/update-available.svg"
import FloatDialog from "./FloatDialog"
import { usePathBuilder } from "./PathBuilder"
import ShortcutsDialog from "./ShortcutsDialog"
import { SnapshotAction, useSnapshotAction } from "./snapshot"
import { combinedStatus } from "./status"
import { AnimDuration, Color, FontSize, SizeUnit } from "./style-helpers"
import { ResourceStatus } from "./types"
import UpdateDialog, { showUpdate } from "./UpdateDialog"

type OverviewResourceBarProps = {
  view: Proto.webviewView
}

let OverviewResourceBarRoot = styled.div`
  display: flex;
  width: 100%;
  border-bottom: 1px dotted ${Color.grayLight};
  justify-content: center;
  align-items: stretch;
`

let ResourceBarEndRoot = styled.div`
  flex-shrink: 1;
  width: 50%;
  display: flex;
  align-items: stretch;
  justify-content: flex-end;
`

let ResourceBarStatusRoot = styled.div`
  display: flex;
  display: flex;
  border-radius: 4px;

  align-items: center;
  flex-grow: 0;
  white-space: nowrap;
  margin: 8px;
  padding: 4px 16px;
`

type ResourceBarStatusProps = {
  view: Proto.webviewView
}

// If there are more than 20 resources,
// use a scale of 1 box = 5%, displaying at least
// one box if count is non-zero.
function boxCount(count: number, total: number): number {
  if (total <= 20) {
    return count
  }

  if (count === 0) {
    return 0
  }

  let pct = count / total
  if (pct <= 0.05) {
    return 1
  }
  return 20 * pct
}

const StatusSquare = styled.div`
  width: 8px;
  height: 8px;
  margin: 0 1px;
  transition: background-color 300ms ease;
`

const GreenSquare = styled(StatusSquare)`
  background-color: ${Color.green};
`
const RedSquare = styled(StatusSquare)`
  background-color: ${Color.red};
`
const GraySquare = styled(StatusSquare)`
  background-color: ${Color.grayLightest};
`

function ResourceBarStatus(props: ResourceBarStatusProps) {
  // Count the statuses.
  let resources = props.view.resources || []
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

  // Summarize the statuses
  let greenSquareCount = boxCount(healthyStatusCount, allStatusCount)
  let redSquareCount = boxCount(unhealthyStatusCount, allStatusCount)
  let graySquareCount = boxCount(pendingStatusCount, allStatusCount)
  let boxes = []
  let extraMargin = { marginLeft: "3px" }
  for (let i = 0; i < greenSquareCount; i++) {
    let style =
      boxes.length % 4 === 0 && boxes.length > 0 ? extraMargin : undefined
    boxes.push(<GreenSquare key={"green-" + i} style={style} />)
  }
  for (let i = 0; i < redSquareCount; i++) {
    let style =
      boxes.length % 4 === 0 && boxes.length > 0 ? extraMargin : undefined
    boxes.push(<RedSquare key={"red-" + i} style={style} />)
  }
  for (let i = 0; i < graySquareCount; i++) {
    let style =
      boxes.length % 4 === 0 && boxes.length > 0 ? extraMargin : undefined
    boxes.push(<GraySquare key={"gray-" + i} style={style} />)
  }

  let summaryMsg =
    `${healthyStatusCount}/${allStatusCount} up ` +
    `| ${unhealthyStatusCount} error${unhealthyStatusCount != 1 ? "s" : ""} ` +
    `| ${warningCount} warning${warningCount != 1 ? "s" : ""}`
  return (
    <ResourceBarStatusRoot>
      {boxes}
      <div style={{ marginLeft: "16px" }}>{summaryMsg}</div>
    </ResourceBarStatusRoot>
  )
}

let MenuButton = styled.button`
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  align-items: center;
  border: 0;
  cursor: pointer;
  background-color: transparent;
  position: relative;
  color: ${Color.blue};
  transition: color ${AnimDuration.default} ease;
  margin: 2px ${SizeUnit(0.75)} 2px 0;
  padding-bottom: 14px;

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
    mouse-events: none;
    cursor: default;
  }
`

let MenuButtonLabel = styled.div`
  position: absolute;
  bottom: 0;
  opacity: 0;
  transition: opacity ${AnimDuration.default} ease;
  font-size: ${FontSize.smallester};
  margin-top: 4px;
  width: 300%;

  ${MenuButton}:hover &,
  ${MenuButton}[data-open="true"] & {
    opacity: 1;
  }
`

let UpdateAvailableFloatIcon = styled(UpdateAvailableIcon)`
  position: absolute;
  top: 0;
  left: -4px;
  display: none;
  width: 10px;
  height: 10px;

  ${MenuButton}.is-enabled & {
    display: block;
  }
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
}

function ResourceBarEnd(props: ResourceBarEndProps) {
  const shortcutButton = useRef(null as any)
  const accountButton = useRef(null as any)
  const updateButton = useRef(null as any)
  const [shortcutsDialogAnchor, setShortcutsDialogAnchor] = useState(
    null as Element | null
  )
  const [accountMenuAnchor, setAccountMenuAnchor] = useState(
    null as Element | null
  )
  const [updateDialogAnchor, setUpdateDialogAnchor] = useState(
    null as Element | null
  )
  const shortcutsDialogOpen = !!shortcutsDialogAnchor
  const accountMenuOpen = !!accountMenuAnchor
  const updateDialogOpen = !!updateDialogAnchor
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

  let accountMenuHeader = <AccountMenuHeader {...props} />
  let accountMenuContent = <AccountMenuContent {...props} />
  let snapshotButton = props.snapshot.enabled ? (
    <MenuButton onClick={props.snapshot.openModal}>
      <SnapshotIcon width="24" height="24" />
      <MenuButtonLabel>{"Create Snapshot"}</MenuButtonLabel>
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
          {props.showUpdate ? "New Version Available" : "Version Settings"}
        </MenuButtonLabel>
      </MenuButton>

      {snapshotButton}

      <MenuButton
        ref={shortcutButton}
        onClick={() => toggleShortcutsDialog("click")}
        data-open={shortcutsDialogOpen}
      >
        <HelpIcon width="24" height="24" />
        <MenuButtonLabel>{"Help"}</MenuButtonLabel>
      </MenuButton>
      <MenuButton
        ref={accountButton}
        onClick={() => toggleAccountMenu("click")}
        data-open={accountMenuOpen}
      >
        <AccountIcon width="24" height="24" />
        <MenuButtonLabel>{"Account"}</MenuButtonLabel>
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
  let resourceBarEndProps = {
    isSnapshot,
    snapshot,
    showUpdate: showUpdate(view),
    suggestedVersion,
    runningBuild,
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
