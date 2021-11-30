import React from "react"
import styled from "styled-components"
import { ApiButton, ApiIcon, buttonsForComponent } from "./ApiButton"
import { MenuButtonLabeled, MenuButtonMixin } from "./GlobalNav"
import { Color, SizeUnit } from "./style-helpers"

type CustomNavProps = {
  view: Proto.webviewView
}

const CustomNavButton = styled(ApiButton)`
  height: 100%;
  align-items: center;

  button {
    ${MenuButtonMixin};
    height: 100%;
    box-shadow: unset;
    justify-content: center;

    &:hover,
    &:active {
      box-shadow: unset;
    }
  }

  .MuiButton-contained.Mui-disabled {
    color: ${Color.blue};
    background: transparent;
  }
  // If there is an options toggle, remove padding between the submit
  // button and the options button.
  button:first-child {
    padding-right: 0px;
  }
  // If there is no options toggle, then restore the default padding.
  button:only-child {
    padding-right: ${SizeUnit(0.5)};
  }
  .apibtn-label {
    display: none;
  }
`

export function CustomNav(props: CustomNavProps) {
  const buttons = buttonsForComponent(
    props.view.uiButtons,
    "global",
    "nav"
  ).default

  return (
    <React.Fragment>
      {buttons.map((b) => (
        <MenuButtonLabeled label={b.spec?.text} key={b.metadata?.name}>
          <CustomNavButton
            uiButton={b}
            variant="contained"
            aria-label={b.spec?.text}
          >
            <ApiIcon
              iconName={b.spec?.iconName || "smart_button"}
              iconSVG={b.spec?.iconSVG}
            />
          </CustomNavButton>
        </MenuButtonLabeled>
      ))}
    </React.Fragment>
  )
}
