import React, { Component, useRef, useState } from "react"
import styled from "styled-components"
import { AccountMenuContent, AccountMenuHeader } from "./AccountMenu"
import { incr } from "./analytics"
import { ReactComponent as AccountIcon } from "./assets/svg/account.svg"
import { ReactComponent as HelpIcon } from "./assets/svg/help.svg"
import { ReactComponent as MetricsIcon } from "./assets/svg/metrics.svg"
import { ReactComponent as SnapshotIcon } from "./assets/svg/snapshot.svg"
import { ReactComponent as UpdateAvailableIcon } from "./assets/svg/update-available.svg"
import FloatDialog from "./FloatDialog"
import MetricsDialog from "./MetricsDialog"
import ShortcutsDialog from "./ShortcutsDialog"
import { SnapshotAction } from "./snapshot"
import {
  AnimDuration,
  Color,
  FontSize,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"
import UpdateDialog from "./UpdateDialog"

const GlobalNavRoot = styled.div`
  flex-grow: 1;
  display: flex;
  align-items: stretch;
  justify-content: flex-end;
`
export const MenuButton = styled.button`
  ${mixinResetButtonStyle};
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
const MenuButtonLabel = styled.div`
  position: absolute;
  bottom: 0;
  font-size: ${FontSize.smallest};
  color: ${Color.blueDark};
  width: 200%;
  transition: opacity ${AnimDuration.default} ease;
  opacity: 0;

  ${MenuButton}:hover &,
  ${MenuButton}[data-open="true"] & {
    opacity: 1;
  }
`
const UpdateAvailableFloatIcon = styled(UpdateAvailableIcon)`
  display: none;
  position: absolute;
  top: 15px;
  left: 5px;
  width: 10px;
  height: 10px;

  &.is-visible {
    display: block;
  }
`

type GlobalNavShortcutsProps = {
  toggleShortcutsDialog: () => void
  snapshot: SnapshotAction
}

/**
 * Sets up keyboard shortcuts that depend on the tilt menu.
 */
class GlobalNavShortcuts extends Component<GlobalNavShortcutsProps> {
  constructor(props: GlobalNavShortcutsProps) {
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

type MetricsServing = Proto.webviewMetricsServing

type GlobalNavProps = {
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

export function GlobalNav(props: GlobalNavProps) {
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
    <GlobalNavRoot>
      <MenuButton
        ref={updateButton}
        onClick={() => toggleUpdateDialog("click")}
        data-open={updateDialogOpen}
      >
        <div>v{props.runningBuild?.version || "?"}</div>

        <UpdateAvailableFloatIcon
          className={props.showUpdate ? "is-visible" : ""}
        />
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
      <GlobalNavShortcuts
        toggleShortcutsDialog={() => toggleShortcutsDialog("shortcut")}
        snapshot={props.snapshot}
      />
    </GlobalNavRoot>
  )
}
