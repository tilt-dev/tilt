import React, { Component, useRef, useState } from "react"
import styled from "styled-components"
import { AccountMenuContent, AccountMenuHeader } from "./AccountMenu"
import { AnalyticsAction, AnalyticsType, incr } from "./analytics"
import { ReactComponent as AccountIcon } from "./assets/svg/account.svg"
import { ReactComponent as HelpIcon } from "./assets/svg/help.svg"
import { ReactComponent as SnapshotIcon } from "./assets/svg/snapshot.svg"
import { ReactComponent as UpdateAvailableIcon } from "./assets/svg/update-available.svg"
import FloatDialog from "./FloatDialog"
import { isTargetEditable } from "./shortcut"
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

type TiltBuild = Proto.corev1alpha1TiltBuild

export const GlobalNavRoot = styled.div`
  display: flex;
  align-items: stretch;
`
export const MenuButtonLabel = styled.div`
  position: absolute;
  bottom: 0;
  font-size: ${FontSize.smallest};
  color: ${Color.blueDark};
  transition: opacity ${AnimDuration.default} ease;
  opacity: 0;
  white-space: nowrap;
  width: 100%;
  text-align: center;
`
export const MenuButtonMixin = `
  ${mixinResetButtonStyle};
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  transition: color ${AnimDuration.default} ease;
  padding-top: ${SizeUnit(0.5)};
  padding-left: ${SizeUnit(0.5)};
  padding-right: ${SizeUnit(0.5)};
  padding-bottom: ${SizeUnit(0.5)};
  font-size: ${FontSize.smallest};
  color: ${Color.blue};
  height: 100%;

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

  &:disabled {
    opacity: 0.33;
  }
`
export const MenuButton = styled.button`
  ${MenuButtonMixin};
`
export const MenuButtonLabeledRoot = styled.div`
  position: relative; // Anchor MenuButtonLabel, which shouldn't affect this element's width
  &:is(:hover, :focus, :active)
    ${MenuButtonLabel},
    button[data-open="true"]
    + ${MenuButtonLabel} {
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
    if (isTargetEditable(e)) {
      return
    }
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

export function MenuButtonLabeled(
  props: React.PropsWithChildren<{ label?: string }>
) {
  return (
    <MenuButtonLabeledRoot>
      {props.children}
      {props.label && <MenuButtonLabel>{props.label}</MenuButtonLabel>}
    </MenuButtonLabeledRoot>
  )
}

type GlobalNavProps = {
  isSnapshot: boolean
  tiltCloudUsername: string
  tiltCloudSchemeHost: string
  tiltCloudTeamID: string
  tiltCloudTeamName: string
  snapshot: SnapshotAction
  showUpdate: boolean
  suggestedVersion: string | null | undefined
  runningBuild: TiltBuild | undefined
}

export function GlobalNav(props: GlobalNavProps) {
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

  let toggleAccountMenu = (action: AnalyticsAction) => {
    if (!accountMenuOpen) {
      incr("ui.web.menu", { type: AnalyticsType.Account, action: action })
    }
    setAccountMenuAnchor(
      accountMenuOpen ? null : (accountButton.current as Element)
    )
  }

  let toggleShortcutsDialog = (action: AnalyticsAction) => {
    if (!shortcutsDialogOpen) {
      incr("ui.web.menu", { type: AnalyticsType.Shortcut, action: action })
    }
    setShortcutsDialogAnchor(
      shortcutsDialogOpen ? null : (shortcutButton.current as Element)
    )
  }

  let toggleUpdateDialog = (action: AnalyticsAction) => {
    if (!updateDialogOpen) {
      incr("ui.web.menu", { type: AnalyticsType.Update, action: action })
    }
    setUpdateDialogAnchor(
      updateDialogOpen ? null : (updateButton.current as Element)
    )
  }

  let accountMenuHeader = <AccountMenuHeader {...props} />
  let accountMenuContent = <AccountMenuContent {...props} />
  let snapshotButton = props.snapshot.enabled ? (
    <MenuButtonLabeled label="Snapshot">
      <MenuButton onClick={props.snapshot.openModal}>
        <SnapshotIcon width="24" height="24" />
      </MenuButton>
    </MenuButtonLabeled>
  ) : null

  const versionButtonLabel = props.showUpdate ? "Get Update" : "Version"

  return (
    <GlobalNavRoot>
      <MenuButtonLabeled label={versionButtonLabel}>
        <MenuButton
          ref={updateButton}
          onClick={() => toggleUpdateDialog(AnalyticsAction.Click)}
          data-open={updateDialogOpen}
          aria-label={versionButtonLabel}
        >
          <div>v{props.runningBuild?.version || "?"}</div>

          <UpdateAvailableFloatIcon
            className={props.showUpdate ? "is-visible" : ""}
          />
        </MenuButton>
      </MenuButtonLabeled>

      {snapshotButton}

      <MenuButtonLabeled label="Help">
        <MenuButton
          ref={shortcutButton}
          onClick={() => toggleShortcutsDialog(AnalyticsAction.Click)}
          data-open={shortcutsDialogOpen}
          aria-label="Help"
        >
          <HelpIcon width="24" height="24" />
        </MenuButton>
      </MenuButtonLabeled>
      <MenuButtonLabeled label="Account">
        <MenuButton
          ref={accountButton}
          onClick={() => toggleAccountMenu(AnalyticsAction.Click)}
          data-open={accountMenuOpen}
          aria-label="Account"
        >
          <AccountIcon width="24" height="24" />
        </MenuButton>
      </MenuButtonLabeled>

      <FloatDialog
        id="accountMenu"
        title={accountMenuHeader}
        open={accountMenuOpen}
        anchorEl={accountMenuAnchor}
        onClose={() => toggleAccountMenu(AnalyticsAction.Close)}
      >
        {accountMenuContent}
      </FloatDialog>
      <ShortcutsDialog
        open={shortcutsDialogOpen}
        anchorEl={shortcutsDialogAnchor}
        onClose={() => toggleShortcutsDialog(AnalyticsAction.Close)}
        isOverview={true}
      />
      <UpdateDialog
        open={updateDialogOpen}
        anchorEl={updateDialogAnchor}
        onClose={() => toggleUpdateDialog(AnalyticsAction.Close)}
        showUpdate={props.showUpdate}
        suggestedVersion={props.suggestedVersion}
        isNewInterface={true}
      />
      <GlobalNavShortcuts
        toggleShortcutsDialog={() =>
          toggleShortcutsDialog(AnalyticsAction.Shortcut)
        }
        snapshot={props.snapshot}
      />
    </GlobalNavRoot>
  )
}
