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

let SidebarAccountRoot = styled.div`
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
let SidebarAccountLabel = styled.p`
  font-family: ${Font.monospace};
  font-size: ${FontSize.small};
  color: ${Color.grayLightest};
  margin-right: ${SizeUnit(0.5)};
  opacity: 0;
  transition: opacity ${AnimDuration.short} ease;
  transition-delay: ${AnimDuration.default};
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

  &:hover ${SidebarAccountLabel} {
    opacity: 1;
  }
`
let SidebarAccountIcon = styled(AccountIcon)`
  fill: ${Color.blue};
  transition: color ${AnimDuration.default} linear;

  &:hover {
    fill: ${Color.blueLight};
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

  &.isVisible {
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
  &.is-signedOut {
  }
`
let MenuContentTeamInTiltfile = styled.small`
  opacity: 0;
  margin-top: 0;
  display: block;
  transition: opacity ${AnimDuration.default} ease;
  transition-delay: ${AnimDuration.short};
`
let MenuContentTeamName = styled.strong`
  transition: background-color ${AnimDuration.default} ease;
  transition-delay: ${AnimDuration.short};
`
let MenuContentTeam = styled.p`
  &:hover {
    ${MenuContentTeamInTiltfile} {
      opacity: 1;
    }
    ${MenuContentTeamName} {
      background-color: ${Color.offWhite};
    }
  }
`
let MenuContentSignInLink = styled.p`
  text-align: center;
`
let MenuContentButtonLink = styled(ButtonLink)`
  margin-top: ${SizeUnit(0.3)};
`
let MenuContentButtonInput = styled(ButtonInput)`
  margin-top: ${SizeUnit(0.5)};
  margin-bottom: ${SizeUnit(0.25)};
`

type SidebarAccountProps = {
  tiltCloudUsername: string | null
  tiltCloudSchemeHost: string
  tiltCloudTeamID: string | null
}

function notifyTiltOfRegistration() {
  let url = `${window.location.protocol}//${window.location.host}/api/user_started_tilt_cloud_registration`
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
      <SidebarAccountMenu className={accountMenuIsVisible ? "isVisible" : ""}>
        <SidebarAccountMenuHeader>
          <SidebarAccountMenuLogo></SidebarAccountMenuLogo>
          {!props.tiltCloudUsername && (
            <SidebarAccountMenuLearn
              href={props.tiltCloudSchemeHost}
              target="_blank"
              rel="noopener noreferrer nofollow"
            >
              Learn More
            </SidebarAccountMenuLearn>
          )}
        </SidebarAccountMenuHeader>
        {props.tiltCloudUsername ? (
          <SidebarAccountMenuContent className="is-signedIn">
            <p>
              Signed in as <strong>{props.tiltCloudUsername}</strong>
            </p>
            <MenuContentTeam>
              On team{" "}
              <MenuContentTeamName>{props.tiltCloudTeamID}</MenuContentTeamName>
              <br />
              <MenuContentTeamInTiltfile>
                From <strong>`set_team()`</strong> in Tiltfile
              </MenuContentTeamInTiltfile>
            </MenuContentTeam>
            <MenuContentButtonLink
              href={props.tiltCloudSchemeHost}
              label="View Tilt Cloud"
              target="_blank"
              rel="noopener noreferrer nofollow"
            />
          </SidebarAccountMenuContent>
        ) : (
          <SidebarAccountMenuContent className="is-signedOut">
            <p>
              Tilt Cloud is a platform for making all kinds of data from Tilt
              available to your team — and making your team’s data available to
              you.
            </p>
            <form
              action={props.tiltCloudSchemeHost + "/start_register_token"}
              target="_blank"
              method="POST"
              onSubmit={notifyTiltOfRegistration}
            >
              <input
                name="token"
                type="hidden"
                value={cookies.get("Tilt-Token")}
              />
              <MenuContentButtonInput
                type="submit"
                value="Sign Up via GitHub"
              />
            </form>
            <MenuContentSignInLink>
              Or{" "}
              <strong>
                <a href={props.tiltCloudSchemeHost}>Sign In</a>
              </strong>{" "}
              to your account.
            </MenuContentSignInLink>
          </SidebarAccountMenuContent>
        )}
      </SidebarAccountMenu>
    </SidebarAccountRoot>
  )
}

export default SidebarAccount
