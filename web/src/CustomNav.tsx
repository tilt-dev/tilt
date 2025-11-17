import React from "react"
import styled from "styled-components"
import {
  ApiButton,
  ApiButtonType,
  ApiIcon,
  buttonsForComponent,
  UIBUTTON_GLOBAL_COMPONENT_ID,
} from "./ApiButton"
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
  .apibtn-label {
    display: none;
  }
`

export function CustomNav(props: CustomNavProps) {
  const buttons = buttonsForComponent(
    props.view.uiButtons,
    ApiButtonType.Global,
    UIBUTTON_GLOBAL_COMPONENT_ID
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
