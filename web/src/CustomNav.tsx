import React from "react"
import styled from "styled-components"
import { ApiButton, ApiIcon, buttonsForComponent } from "./ApiButton"
import { MenuButtonLabel, MenuButtonMixin } from "./GlobalNav"
import { SizeUnit } from "./style-helpers"

type CustomNavProps = {
  view: Proto.webviewView
}

const CustomNavButton = styled(ApiButton)`
  align-items: center;

  button {
    ${MenuButtonMixin};
  }
  // If there is an options toggle, remove padding between the submit
  // button and the options button.
  button:first-child {
    padding-right: 0px;
  }
  // If there is no options toggle, then restore the default padding.
  button:only-child {
    ${SizeUnit(0.5)};
  }
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
