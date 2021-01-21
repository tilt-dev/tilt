import React, { Component, useRef, useState } from "react"
import ReactOutlineManager from "react-outline-manager"
import styled from "styled-components"
import { AccountMenuContent, AccountMenuHeader } from "./AccountMenu"
import { incr } from "./analytics"
import { ReactComponent as AccountIcon } from "./assets/svg/account.svg"
import { ReactComponent as HelpIcon } from "./assets/svg/help.svg"
import FloatDialog from "./FloatDialog"
import ShortcutsDialog from "./ShortcutsDialog"
import { AnimDuration, Color, SizeUnit } from "./style-helpers"
import UpdateDialog from "./UpdateDialog"

export const SidebarAccountRoot = styled.div`
  position: relative; // Anchor SidebarAccountMenu
  width: 100%;
`
let SidebarAccountHeader = styled.header`
  display: flex;
  justify-content: flex-end;
  padding-top: ${SizeUnit(0.5)};
  padding-left: ${SizeUnit(0.5)};
  padding-right: ${SizeUnit(0.5)};
  align-items: flex-end;
`
let SidebarAccountButton = styled.button`
  display: flex;
  align-items: center;
  border: 0;
  cursor: pointer;
  background-color: transparent;
  padding-left: ${SizeUnit(0.25)};
  padding-right: ${SizeUnit(0.25)};
  padding-top: ${SizeUnit(0.15)};
  padding-bottom: ${SizeUnit(0.15)};
  position: relative; // Anchor SidebarAccountLabel
  color: ${Color.blue};
  transition: color ${AnimDuration.default} linear;

  &:hover {
    color: ${Color.blueLight};
  }
`
let SidebarAccountIcon = styled(AccountIcon)`
  fill: ${Color.blue};
  transition: fill ${AnimDuration.default} linear;

  &:hover {
    fill: ${Color.blueLight};
  }
`
let SidebarHelpIcon = styled(HelpIcon)`
  fill: ${Color.blue};
  transition: fill ${AnimDuration.default} linear;

  &:hover {
    fill: ${Color.blueLight};
  }
`
type SidebarAccountProps = {
  isSnapshot: boolean
  tiltCloudUsername: string | null
  tiltCloudSchemeHost: string
  tiltCloudTeamID: string | null
  tiltCloudTeamName: string | null
}

/**
 * Sets up keyboard shortcuts that depend on the sidebar account block.
 */
class SidebarAccountShortcuts extends Component<{
  toggleShortcutsDialog: () => void
}> {
  constructor(props: { toggleShortcutsDialog: () => void }) {
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
    }
  }

  render() {
    return <span></span>
  }
}

function SidebarAccount(props: SidebarAccountProps) {
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
  if (props.isSnapshot) {
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

  return (
    <SidebarAccountRoot>
      <ReactOutlineManager>
        <SidebarAccountHeader>
          <SidebarAccountButton
            ref={updateButton}
            onClick={() => toggleUpdateDialog("click")}
          >
            <div>New UI</div>
          </SidebarAccountButton>
          <SidebarAccountButton
            ref={shortcutButton}
            onClick={() => toggleShortcutsDialog("click")}
          >
            <SidebarHelpIcon width="20" height="20" />
          </SidebarAccountButton>
          <SidebarAccountButton
            ref={accountButton}
            onClick={() => toggleAccountMenu("click")}
          >
            <SidebarAccountIcon width="20" height="20" />
          </SidebarAccountButton>
        </SidebarAccountHeader>
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
          isOverview={false}
        />
        <SidebarAccountShortcuts
          toggleShortcutsDialog={() => toggleShortcutsDialog("shortcut")}
        />
        <UpdateDialog
          open={updateDialogOpen}
          anchorEl={updateDialogAnchor}
          onClose={() => toggleUpdateDialog("close")}
          showUpdate={false}
          suggestedVersion={""}
          isNewInterface={false}
        />
      </ReactOutlineManager>
    </SidebarAccountRoot>
  )
}

export default SidebarAccount
