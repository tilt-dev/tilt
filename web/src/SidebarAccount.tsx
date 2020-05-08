import React, { useState } from "react"
import cookies from "js-cookie"
import styled, { keyframes } from "styled-components"
import {
  Color,
  Font,
  FontSize,
  SizeUnit,
  AnimDuration,
  ZIndex,
} from "./style-helpers"
import { ReactComponent as AccountIcon } from "./assets/svg/account.svg"
import { ReactComponent as TiltCloudLogoSvg } from "./assets/svg/logo-Tilt-Cloud.svg"
import ButtonLink from "./ButtonLink"
import ButtonInput from "./ButtonInput"

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
let SidebarAccountLabel = styled.p`
  position: absolute;
  font-family: ${Font.monospace};
  font-size: ${FontSize.small};
  color: ${Color.grayLightest};
  margin-right: ${SizeUnit(0.25)};
  opacity: 0;
  right: 100%;
  transition: opacity ${AnimDuration.short} ease;
  white-space: nowrap;

  ${SidebarAccountButton}:hover & {
    transition-delay: ${AnimDuration.long};
    opacity: 1;
  }
`
let scaleWithBounce = keyframes`
    0% {
        transform: scaleY(0)
    }
    80% {
        transform: scaleY(1.05)
    }
    100% {
        transform: scaleY(1)
    }
`
let SidebarAccountMenu = styled.div`
  position: absolute;
  left: 0;
  right: 0;
  background-color: ${Color.white};
  color: ${Color.text};
  border-radius: ${SizeUnit(0.25)};
  margin-top: ${SizeUnit(0.25)};
  margin-left: ${SizeUnit(0.15)};
  margin-right: ${SizeUnit(0.15)};
  z-index: ${ZIndex.SidebarMenu};
  transform: scaleY(0);
  opacity: 0;
  transform-origin: top center;
  transition: transform ${AnimDuration.short} ease,
    opacity ${AnimDuration.short} linear;

  &.is-visible {
    transform: scaleY(1);
    animation: ${scaleWithBounce} 300ms ease-in-out;
    opacity: 1;
  }
`
let SidebarAccountMenuHeader = styled.header`
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px dotted ${Color.grayLight};
  padding-top: ${SizeUnit(0.25)};
  padding-bottom: ${SizeUnit(0.25)};
  padding-left: ${SizeUnit(0.5)};
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
  font-family: ${Font.monospace};
  font-size: ${FontSize.default};
  color: ${Color.grayLight};
  padding: ${SizeUnit(0.5)};

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

function SidebarAccount(props: SidebarAccountProps) {
  const [accountMenuIsVisible, toggleAccountMenu] = useState(false)

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

  let signedInMenuContent = (
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

  let signedOutMenuContent = (
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

  if (props.isSnapshot) {
    return null
  }

  return (
    <SidebarAccountRoot>
      <SidebarAccountHeader>
        <SidebarAccountButton
          onClick={() => toggleAccountMenu(!accountMenuIsVisible)}
        >
          <SidebarAccountLabel>Your Tilt Cloud status</SidebarAccountLabel>
          <SidebarAccountIcon />
        </SidebarAccountButton>
      </SidebarAccountHeader>
      <SidebarAccountMenu className={accountMenuIsVisible ? "is-visible" : ""}>
        <SidebarAccountMenuHeader>
          <SidebarAccountMenuLogo></SidebarAccountMenuLogo>
          {optionalLearnMore}
        </SidebarAccountMenuHeader>
        {props.tiltCloudUsername ? signedInMenuContent : signedOutMenuContent}
      </SidebarAccountMenu>
    </SidebarAccountRoot>
  )
}

export default SidebarAccount
