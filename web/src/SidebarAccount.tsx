import React, { useState, Component } from "react"
import cookies from "js-cookie"
import styled from "styled-components"
import { Color, Font, FontSize, SizeUnit, AnimDuration } from "./style-helpers"
import { ReactComponent as AccountIcon } from "./assets/svg/account.svg"
import { ReactComponent as HelpIcon } from "./assets/svg/help.svg"
import { ReactComponent as TiltCloudLogoSvg } from "./assets/svg/logo-Tilt-Cloud.svg"
import ButtonLink from "./ButtonLink"
import ButtonInput from "./ButtonInput"
import ReactOutlineManager from "react-outline-manager"
import FloatDialog from "./FloatDialog"
import ShortcutsDialog from "./ShortcutsDialog"
import { incr } from "./analytics"

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
let SidebarAccountMenuHeader = styled.div`
  display: flex;
  flex-grow: 1;
  align-items: center;
  justify-content: space-between;
  padding-right: ${SizeUnit(0.5)};
`
let SidebarAccountMenuLogo = styled(TiltCloudLogoSvg)`
  fill: ${Color.text};
`
let SidebarAccountMenuLearn = styled.a`
  font-family: ${Font.sansSerif};
  font-size: ${FontSize.smallest};
  color: ${Color.grayLight};
`
let SidebarAccountMenuContent = styled.div`
  color: ${Color.grayLight};

  p + p {
    margin-top: ${SizeUnit(0.25)};
  }

  strong {
    font-weight: bold;
    color: ${Color.text};
  }
  small {
    font-size: ${FontSize.small};
  }
  &.is-signedIn {
    margin-top: ${SizeUnit(0.3)};
    text-align: right;
  }
`

let MenuContentTeam = styled.p``
let MenuContentTeamName = styled.strong`
  transition: background-color ${AnimDuration.default} ease;
  transition-delay: ${AnimDuration.short};

  ${MenuContentTeam}:hover & {
    background-color: ${Color.offWhite};
  }
`
let MenuContentTeamInTiltfile = styled.small`
  opacity: 0;
  margin-top: 0;
  display: block;
  transition: opacity ${AnimDuration.default} ease;
  transition-delay: ${AnimDuration.short};

  ${MenuContentTeam}:hover & {
    opacity: 1;
  }
`
export const MenuContentButtonTiltCloud = styled(ButtonLink)`
  margin-top: ${SizeUnit(0.3)};
`
export const MenuContentButtonSignUp = styled(ButtonInput)`
  margin-top: ${SizeUnit(0.5)};
  margin-bottom: ${SizeUnit(0.25)};
`

type SidebarAccountProps = {
  isSnapshot: boolean
  tiltCloudUsername: string | null
  tiltCloudSchemeHost: string
  tiltCloudTeamID: string | null
  tiltCloudTeamName: string | null
}

function notifyTiltOfRegistration() {
  let url = `/api/user_started_tilt_cloud_registration`
  fetch(url, {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
  })
}

function SidebarMenuContent(props: SidebarAccountProps) {
  if (props.tiltCloudUsername) {
    let teamContent = null
    if (props.tiltCloudTeamID) {
      teamContent = (
        <MenuContentTeam>
          On team{" "}
          <MenuContentTeamName>
            {props.tiltCloudTeamName ?? props.tiltCloudTeamID}
          </MenuContentTeamName>
          <br />
          <MenuContentTeamInTiltfile>
            From <strong>`set_team()`</strong> in Tiltfile
          </MenuContentTeamInTiltfile>
        </MenuContentTeam>
      )
    }

    return (
      <SidebarAccountMenuContent className="is-signedIn">
        <p>
          Signed in as <strong>{props.tiltCloudUsername}</strong>
        </p>
        {teamContent}
        <MenuContentButtonTiltCloud
          href={props.tiltCloudSchemeHost}
          target="_blank"
          rel="noopener noreferrer nofollow"
        >
          View Tilt Cloud
        </MenuContentButtonTiltCloud>
      </SidebarAccountMenuContent>
    )
  }
  return (
    <SidebarAccountMenuContent>
      <p>
        Tilt Cloud is a platform for making all kinds of data from Tilt
        available to your team — and making your team’s data available to you.
      </p>
      <form
        action={props.tiltCloudSchemeHost + "/start_register_token"}
        target="_blank"
        method="POST"
        onSubmit={notifyTiltOfRegistration}
      >
        <input name="token" type="hidden" value={cookies.get("Tilt-Token")} />
        <MenuContentButtonSignUp
          type="submit"
          value="Link Tilt to Tilt Cloud"
        />
      </form>
    </SidebarAccountMenuContent>
  )
}

export { SidebarMenuContent }

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

  let optionalLearnMore = null
  if (!props.tiltCloudUsername) {
    optionalLearnMore = (
      <SidebarAccountMenuLearn
        href={props.tiltCloudSchemeHost}
        target="_blank"
        rel="noopener noreferrer nofollow"
      >
        Learn More
      </SidebarAccountMenuLearn>
    )
  }

  if (props.isSnapshot) {
    return null
  }

  let accountMenuHeader = (
    <SidebarAccountMenuHeader>
      <SidebarAccountMenuLogo></SidebarAccountMenuLogo>
      {optionalLearnMore}
    </SidebarAccountMenuHeader>
  )

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
          <SidebarMenuContent {...props} />
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
