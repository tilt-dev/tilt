import React from "react"
import styled from "styled-components"
import { ReactComponent as TriggerButtonManualSvg } from "./assets/svg/trigger-button-manual.svg"
import { ReactComponent as TriggerButtonSvg } from "./assets/svg/trigger-button.svg"
import { AnimDuration, Color, Width } from "./style-helpers"
import { TriggerMode } from "./types"

export let SidebarTriggerButtonRoot = styled.button`
  background-color: ${Color.gray};
  border-radius: 50% 0 0 50%;
  border: none;
  width: ${Width.sidebarTriggerButton}px;
  height: ${Width.sidebarTriggerButton}px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  box-shadow: inset 0px 0px 8px rgba(0, 0, 0, 0.24);
  margin: 0 0 0 6px;
  opacity: 0;
  pointer-events: none;

  &.clickable {
    pointer-events: auto;
    cursor: pointer;
  }

  &.clickable,
  &.isQueued {
    opacity: 1;
  }

  &.isSelected {
    background-color: ${Color.gray7};
  }
  &:hover {
    background-color: ${Color.grayDark};
  }
  &.isSelected:hover {
    background-color: ${Color.grayLightest};
  }

  & .fillStd {
    transition: fill ${AnimDuration.default} ease;
    fill: ${Color.grayLight};
  }
  &.isSelected .fillStd {
    fill: ${Color.black};
  }
  &:hover .fillStd {
    fill: ${Color.blue};
  }
  &.isSelected:hover .fillStd {
    fill: ${Color.blueDark};
  }
  & > svg {
    transition: transform ${AnimDuration.short} linear;
  }
  &:active > svg {
    transform: scale(1.2);
  }
  &.isQueued > svg {
    animation: spin 1s linear infinite;
  }
`

export const TriggerButtonTooltip = {
  AlreadyQueued:
    "Cannot trigger an update if resource is already queued for update.",
  NeedsManualTrigger: "Click to trigger an update.",
  UpdateInProgOrPending:
    "Cannot trigger an update while resource is updating or update is pending.",
  ClickToForce: "Force an update for this resource.",
}

type SidebarTriggerButtonProps = {
  isTiltfile: boolean
  isBuilding: boolean
  hasBuilt: boolean
  triggerMode: TriggerMode
  isSelected: boolean
  hasPendingChanges: boolean
  isQueued: boolean
  onTrigger: (action: string) => void
}

const titleText = (
  clickable: boolean,
  clickMe: boolean,
  isQueued: boolean
): string => {
  if (isQueued) {
    return TriggerButtonTooltip.AlreadyQueued
  } else if (!clickable) {
    return TriggerButtonTooltip.UpdateInProgOrPending
  } else if (clickMe) {
    return TriggerButtonTooltip.NeedsManualTrigger
  } else {
    return TriggerButtonTooltip.ClickToForce
  }
}

function SidebarTriggerButton(props: SidebarTriggerButtonProps) {
  let isManualTriggerMode = props.triggerMode !== TriggerMode.TriggerModeAuto
  let isManualTriggerIncludingInitial =
    props.triggerMode === TriggerMode.TriggerModeManualIncludingInitial

  // clickable (i.e. trigger button will appear) if it doesn't already have some kind of pending / active build
  let clickable =
    !props.isQueued && // already queued for manual run
    !props.isBuilding && // currently building
    !(!isManualTriggerIncludingInitial && !props.hasBuilt) && // waiting to perform its initial build
    !(props.hasPendingChanges && !isManualTriggerMode) // waiting to perform an auto-triggered build in response to a change

  let clickMe = false
  if (clickable) {
    if (props.hasPendingChanges && isManualTriggerMode) {
      clickMe = true
    } else if (!props.hasBuilt && isManualTriggerIncludingInitial) {
      clickMe = true
    }
  }

  let onClick = (e: any) => {
    // SidebarTriggerButton is nested in a link,
    // and preventDefault is the standard way to cancel the navigation.
    e.preventDefault()

    // stopPropagation prevents the overview card from opening.
    e.stopPropagation()

    props.onTrigger("click")
  }

  // Add padding to center the icon better.
  let padding = clickMe ? "0" : "0 0 0 2px"
  let classes = []
  if (props.isSelected) {
    classes.push("isSelected")
  }
  if (clickable) {
    classes.push("clickable")
  }
  if (props.isQueued) {
    classes.push("isQueued")
  }

  return (
    <SidebarTriggerButtonRoot
      onClick={onClick}
      className={classes.join(" ")}
      disabled={!clickable}
      title={titleText(clickable, clickMe, props.isQueued)}
      style={{ padding }}
    >
      {clickMe ? <TriggerButtonManualSvg /> : <TriggerButtonSvg />}
    </SidebarTriggerButtonRoot>
  )
}

export default React.memo(SidebarTriggerButton)
