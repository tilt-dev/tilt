import cookies from "js-cookie"
import React from "react"
import styled from "styled-components"
import { ReactComponent as TiltCloudLogoSvg } from "./assets/svg/logo-Tilt-Cloud.svg"
import ButtonInput from "./ButtonInput"
import ButtonLink from "./ButtonLink"
import { AnimDuration, Color, Font, FontSize, SizeUnit } from "./style-helpers"

let AccountMenuContentRoot = styled.div`
  color: ${Color.gray50};

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

type AccountMenuProps = {
  isSnapshot: boolean
  tiltCloudUsername: string | null
  tiltCloudSchemeHost: string
  tiltCloudTeamID: string | null
  tiltCloudTeamName: string | null
}

export function AccountMenuContent(props: AccountMenuProps) {
  if (props.tiltCloudUsername) {
    let teamContent = null
    if (props.tiltCloudTeamID) {
      teamContent = (
        <MenuContentTeam>
          On team{" "}
          <MenuContentTeamName>
            {props.tiltCloudTeamName || props.tiltCloudTeamID}
          </MenuContentTeamName>
          <br />
          <MenuContentTeamInTiltfile>
            From <strong>`set_team()`</strong> in Tiltfile
          </MenuContentTeamInTiltfile>
        </MenuContentTeam>
      )
    }

    return (
      <AccountMenuContentRoot className="is-signedIn">
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
      </AccountMenuContentRoot>
    )
  }
  return (
    <AccountMenuContentRoot>
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
    </AccountMenuContentRoot>
  )
}

let AccountMenuHeaderRoot = styled.div`
  display: flex;
  flex-grow: 1;
  align-items: center;
  justify-content: space-between;
  padding-right: ${SizeUnit(0.5)};
`
let AccountMenuLogo = styled(TiltCloudLogoSvg)`
  fill: ${Color.text};
`
let AccountMenuLearn = styled.a`
  font-family: ${Font.sansSerif};
  font-size: ${FontSize.smallest};
  color: ${Color.gray50};
`

export function AccountMenuHeader(props: AccountMenuProps) {
  let optionalLearnMore = null
  if (!props.tiltCloudUsername) {
    optionalLearnMore = (
      <AccountMenuLearn
        href={props.tiltCloudSchemeHost}
        target="_blank"
        rel="noopener noreferrer nofollow"
      >
        Learn More
      </AccountMenuLearn>
    )
  }

  return (
    <AccountMenuHeaderRoot>
      <AccountMenuLogo />
      {optionalLearnMore}
    </AccountMenuHeaderRoot>
  )
}
