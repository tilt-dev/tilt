import React from "react"
import styled, { ThemeProvider } from "styled-components"
import "./SidebarTriggerButton.scss"
import { Width } from "./style-helpers"
import { TriggerMode } from "./types"

let TriggerModeToggleStyle = styled.button`
  background-position: center center;
  background-color: ${(props) =>
    props.theme.isManualTriggerMode ? "violet" : "green"};
  /* -webkit-mask: url("assets/svg/trigger-button.svg") no-repeat 50% 50%;
  mask: url("assets/svg/trigger-button.svg") no-repeat 50% 50%; */
  border: 0 none;
  width: ${Width.sidebarTriggerButton}px;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;

  cursor: pointer;
`

type TriggerModeToggleProps = {
  triggerMode: TriggerMode
  onModeToggle: (mode: TriggerMode) => void
  // TODO: is set from UI? (bool)
}

export const ToggleTriggerModeTooltip = {
  EnableAuto: "Click to enable auto mode",
  DisableAuto: "Click to disable auto mode",
}

const titleText = (isManual: boolean): string => {
  if (isManual) {
    return ToggleTriggerModeTooltip.EnableAuto
  } else {
    return ToggleTriggerModeTooltip.DisableAuto
  }
}

function TriggerModeToggle(props: TriggerModeToggleProps) {
  let isManualTriggerMode = props.triggerMode !== TriggerMode.TriggerModeAuto
  let desiredMode = isManualTriggerMode
    ? TriggerMode.TriggerModeAuto
    : TriggerMode.TriggerModeManualAfterInitial
  let onClick = (e: any) => {
    // TriggerModeToggle is nested in a link,
    // and preventDefault is the standard way to cancel the navigation.
    e.preventDefault()

    // stopPropagation prevents the overview card from opening.
    e.stopPropagation()

    props.onModeToggle(desiredMode)
  }

  let theme = {
    isManualTriggerMode: isManualTriggerMode,
  }
  return (
    <ThemeProvider theme={theme}>
      <TriggerModeToggleStyle
        onClick={onClick}
        title={titleText(isManualTriggerMode)}
      />
    </ThemeProvider>
  )
}

export { TriggerModeToggle, TriggerModeToggleStyle }
