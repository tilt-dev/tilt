import React, { Component, useMemo, useRef, useState } from "react"
import styled from "styled-components"
import { ReactComponent as ClusterErrorIcon } from "./assets/svg/close.svg"
import { ReactComponent as ClusterIcon } from "./assets/svg/cluster-icon.svg"
import { ReactComponent as HelpIcon } from "./assets/svg/help.svg"
import { ReactComponent as SnapshotIcon } from "./assets/svg/snapshot.svg"
import { ReactComponent as UpdateAvailableIcon } from "./assets/svg/update-available.svg"
import { ClusterStatusDialog, getDefaultCluster } from "./ClusterStatusDialog"
import { useFeatures } from "./feature"
import HelpDialog from "./HelpDialog"
import { isTargetEditable } from "./shortcut"
import { SnapshotAction } from "./snapshot"
import {
  AnimDuration,
  Color,
  FontSize,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"
import { Cluster } from "./types"
import UpdateDialog from "./UpdateDialog"
import type { TiltBuild } from "./core"

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
  padding: ${SizeUnit(0.5)};
  font-size: ${FontSize.smallest};
  color: ${Color.blue};
  height: 100%;

  & .fillStd {
    fill: ${Color.blue};
    transition: fill ${AnimDuration.default} ease;
  }
  &:hover .fillStd :not(.has-error) {
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

const floatIconMixin = `
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
const UpdateAvailableFloatIcon = styled(UpdateAvailableIcon)`
  ${floatIconMixin}
`

const ClusterErrorFloatIcon = styled(ClusterErrorIcon)`
  ${floatIconMixin}

  .fillStd,
  &:hover .fillStd {
    fill: ${Color.red};
  }
`

const ClusterStatusIcon = styled(ClusterIcon)`
  &.has-error {
    .fillStd {
      fill: ${Color.red};
    }
  }
`

type GlobalNavShortcutsProps = {
  toggleHelpDialog: () => void
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
      this.props.toggleHelpDialog()
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

export type GlobalNavProps = {
  isSnapshot: boolean
  snapshot: SnapshotAction
  showUpdate: boolean
  suggestedVersion: string | null | undefined
  runningBuild: TiltBuild | undefined
  clusterConnections?: Cluster[]
}

// The snapshot menu item is handled separately in HUD
// since it requires access to HUD state.
enum NavDialog {
  Account = "account",
  Cluster = "cluster",
  Help = "help",
  Update = "update",
}

export function GlobalNav(props: GlobalNavProps) {
  const helpButton = useRef<HTMLButtonElement | null>(null)
  const accountButton = useRef<HTMLButtonElement | null>(null)
  const updateButton = useRef<HTMLButtonElement | null>(null)
  const clusterButton = useRef<HTMLButtonElement | null>(null)
  const snapshotButton = useRef<HTMLButtonElement | null>(null)

  const [openDialog, setOpenDialog] = useState<NavDialog | null>(null)

  const features = useFeatures()

  // Don't display global nav for snapshots
  if (props.isSnapshot) {
    return null
  }

  function toggleDialog(name: NavDialog) {
    const dialogIsOpen = openDialog === name

    const nextDialogState = dialogIsOpen ? null : name
    setOpenDialog(nextDialogState)
  }

  let snapshotMenuItem = props.snapshot.enabled ? (
    <MenuButtonLabeled label="Snapshot">
      <MenuButton
        ref={snapshotButton}
        onClick={() => props.snapshot.openModal(snapshotButton.current)}
        role="menuitem"
        aria-label="Snapshot"
        aria-haspopup="true"
      >
        <SnapshotIcon width="24" height="24" />
      </MenuButton>
    </MenuButtonLabeled>
  ) : null

  // Only display the cluster status menu item if default cluster information is available
  const defaultClusterInfo = useMemo(
    () => getDefaultCluster(props.clusterConnections),
    [props.clusterConnections]
  )
  const clusterStatusButton = defaultClusterInfo ? (
    <MenuButtonLabeled label="Cluster">
      <MenuButton
        ref={clusterButton}
        onClick={() => toggleDialog(NavDialog.Cluster)}
        data-open={openDialog === NavDialog.Cluster}
        aria-expanded={openDialog === NavDialog.Cluster}
        aria-label={`Cluster status: ${
          defaultClusterInfo.status?.error ? "error" : "healthy"
        }`}
        aria-haspopup="true"
        role="menuitem"
      >
        <ClusterErrorFloatIcon
          className={defaultClusterInfo.status?.error && "is-visible has-error"}
          role="presentation"
        />
        <ClusterStatusIcon
          role="presentation"
          className={defaultClusterInfo.status?.error && "has-error"}
          width="24"
          height="24"
        />
      </MenuButton>
    </MenuButtonLabeled>
  ) : null

  const versionButtonLabel = props.showUpdate ? "Get Update" : "Version"

  return (
    <GlobalNavRoot role="menu" aria-label="Tilt session menu">
      {clusterStatusButton}

      <MenuButtonLabeled label={versionButtonLabel}>
        <MenuButton
          ref={updateButton}
          onClick={() => toggleDialog(NavDialog.Update)}
          data-open={openDialog === NavDialog.Update}
          aria-expanded={openDialog === NavDialog.Update}
          aria-label={versionButtonLabel}
          aria-haspopup="true"
          role="menuitem"
        >
          <div>v{props.runningBuild?.version || "?"}</div>

          <UpdateAvailableFloatIcon
            className={props.showUpdate ? "is-visible" : ""}
          />
        </MenuButton>
      </MenuButtonLabeled>

      {snapshotMenuItem}

      <MenuButtonLabeled label="Help">
        <MenuButton
          ref={helpButton}
          onClick={() => toggleDialog(NavDialog.Help)}
          data-open={openDialog === NavDialog.Help}
          aria-expanded={openDialog === NavDialog.Help}
          aria-label="Help"
          aria-haspopup="true"
          role="menuitem"
        >
          <HelpIcon width="24" height="24" />
        </MenuButton>
      </MenuButtonLabeled>

      <ClusterStatusDialog
        open={openDialog === NavDialog.Cluster}
        onClose={() => toggleDialog(NavDialog.Cluster)}
        anchorEl={clusterButton?.current}
        clusterConnection={defaultClusterInfo}
      />
      <HelpDialog
        open={openDialog === NavDialog.Help}
        anchorEl={helpButton?.current}
        onClose={() => toggleDialog(NavDialog.Help)}
      />
      <UpdateDialog
        open={openDialog === NavDialog.Update}
        anchorEl={updateButton?.current}
        onClose={() => toggleDialog(NavDialog.Update)}
        showUpdate={props.showUpdate}
        suggestedVersion={props.suggestedVersion}
        isNewInterface={true}
      />
      <GlobalNavShortcuts
        toggleHelpDialog={() => toggleDialog(NavDialog.Help)}
        snapshot={props.snapshot}
      />
    </GlobalNavRoot>
  )
}
