import { Icon } from "@material-ui/core"
import React from "react"
import styled from "styled-components"
import { ApiButton } from "./ApiButton"
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
  const buttons =
    props.view.uiButtons?.filter(
      (b) =>
        b.spec?.location &&
        (b.spec.location.componentType ?? "").toLowerCase() === "global" &&
        (b.spec.location.componentID ?? "").toLowerCase() === "nav"
    ) || []

  return (
    <React.Fragment>
      {buttons.map((b) => (
        <CustomNavButton key={b.metadata?.name} button={b}>
          <Icon>{b.spec?.iconName}</Icon>
          <MenuButtonLabel>{b.spec?.text}</MenuButtonLabel>
        </CustomNavButton>
      ))}
    </React.Fragment>
  )
}
