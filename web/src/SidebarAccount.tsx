import React, { Component, useState } from "react"
import ReactOutlineManager from "react-outline-manager"
import styled from "styled-components"
import { AccountMenuContent, AccountMenuHeader } from "./AccountMenu"
import { incr } from "./analytics"
import { ReactComponent as AccountIcon } from "./assets/svg/account.svg"
import { ReactComponent as HelpIcon } from "./assets/svg/help.svg"
import FloatDialog from "./FloatDialog"
import ShortcutsDialog from "./ShortcutsDialog"
import { AnimDuration, Color, SizeUnit } from "./style-helpers"

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
  const [shortcutsDialogOpen, setShortcutsDialogOpen] = useState(false)
  const [accountMenuOpen, setAccountMenuOpen] = useState(false)

  let toggleAccountMenu = (action: string) => {
    if (!accountMenuOpen) {
      incr("ui.web.menu", { type: "account", action: action })
    }
    setAccountMenuOpen(!accountMenuOpen)
  }

  let toggleShortcutsDialog = (action: string) => {
    if (!shortcutsDialogOpen) {
      incr("ui.web.menu", { type: "shortcuts", action: action })
    }
    setShortcutsDialogOpen(!shortcutsDialogOpen)
  }

  if (props.isSnapshot) {
    return null
  }

  let accountMenuHeader = <AccountMenuHeader {...props} />
  let accountMenuContent = <AccountMenuContent {...props} />

  // NOTE(nick): A better way to position these would be to re-parent them under
  // SidebarAccountHeader, but to do that we'd need to do some react wiring that
  // I'm not enthusiastic about.
  let accountMenuStyle = {
    content: {
      top: SizeUnit(3),
      right: SizeUnit(0.5),
      position: "absolute",
      width: "400px",
    },
    overlay: { display: "block" },
  }
  let shortcutsDialogStyle = {
    content: {
      top: SizeUnit(3),
      right: SizeUnit(1.5),
      position: "absolute",
      width: "400px",
    },
    overlay: { display: "block" },
  }

  return (
    <SidebarAccountRoot>
      <ReactOutlineManager>
        <SidebarAccountHeader>
          <SidebarAccountButton onClick={() => toggleShortcutsDialog("click")}>
            <SidebarHelpIcon />
          </SidebarAccountButton>
          <SidebarAccountButton onClick={() => toggleAccountMenu("click")}>
            <SidebarAccountIcon />
          </SidebarAccountButton>
        </SidebarAccountHeader>
        <FloatDialog
          title={accountMenuHeader}
          isOpen={accountMenuOpen}
          onRequestClose={() => toggleAccountMenu("close")}
          style={accountMenuStyle}
        >
          {accountMenuContent}
        </FloatDialog>
        <ShortcutsDialog
          isOpen={shortcutsDialogOpen}
          onRequestClose={() => toggleShortcutsDialog("close")}
          style={shortcutsDialogStyle}
        />
        <SidebarAccountShortcuts
          toggleShortcutsDialog={() => toggleShortcutsDialog("shortcut")}
        />
      </ReactOutlineManager>
    </SidebarAccountRoot>
  )
}

export default SidebarAccount
