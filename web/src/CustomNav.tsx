import React from "react"
import styled from "styled-components"
import { ApiButton, ApiIcon, buttonsForComponent } from "./ApiButton"
import { MenuButtonLabel, MenuButtonMixin } from "./GlobalNav"

type CustomNavProps = {
  view: Proto.webviewView
}

const CustomNavButton = styled(ApiButton)`
  ${MenuButtonMixin}
  .apibtn-label {
    display: none;
  }
`

export function CustomNav(props: CustomNavProps) {
  const buttons = buttonsForComponent(props.view.uiButtons, "global", "nav")

  return (
    <React.Fragment>
      {buttons.map((b) => (
        <CustomNavButton key={b.metadata?.name} button={b}>
          <ApiIcon
            iconName={b.spec?.iconName || "smart_button"}
            iconSVG={b.spec?.iconSVG}
          />
          <MenuButtonLabel>{b.spec?.text}</MenuButtonLabel>
        </CustomNavButton>
      ))}
    </React.Fragment>
  )
}
