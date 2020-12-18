import React from "react"
import styled from "styled-components"
import "./SidebarTriggerButton.scss"
import { Width } from "./style-helpers"
import { TriggerMode } from "./types"

let SidebarTriggerButtonStyle = styled.button`
  background-position: center center;
  background-repeat: no-repeat;
  background-color: transparent;
  border: 0 none;
  width: ${Width.sidebarTriggerButton}px;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
`

export const TriggerButtonTooltip = {
  AlreadyQueued:
    "Cannot trigger an update if resource is already queued for update.",
  ManualResourcePendingChanges:
    "This manual resource has pending file changes; click to trigger an update.",
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
    return TriggerButtonTooltip.ManualResourcePendingChanges
  } else {
    return TriggerButtonTooltip.ClickToForce
  }
}

export default React.memo(function SidebarTriggerButton(
  props: SidebarTriggerButtonProps
) {
  let isManualTriggerMode = props.triggerMode !== TriggerMode.TriggerModeAuto

  // clickable (i.e. trigger button will appear) if it doesn't already have some kind of pending / active build
  let clickable =
    !props.isQueued && // already queued for manual run
    !props.isBuilding && // currently building
    !(
      props.triggerMode !== TriggerMode.TriggerModeManualIncludingInitial &&
      !props.hasBuilt
    ) && // waiting to perform its initial build
    !(props.hasPendingChanges && !isManualTriggerMode) // waiting to perform an auto-triggered build in response to a change
  let clickMe = props.hasPendingChanges && isManualTriggerMode
  let onClick = (e: any) => {
    // SidebarTriggerButton is nested in a link,
    // and preventDefault is the standard way to cancel the navigation.
    e.preventDefault()

    // stopPropagation prevents the overview card from opening.
    e.stopPropagation()

    props.onTrigger("click")
  }

  return (
    <SidebarTriggerButtonStyle
      onClick={onClick}
      className={`SidebarTriggerButton ${props.isSelected ? "isSelected" : ""}
          ${clickable ? " clickable" : ""}${clickMe ? " clickMe" : ""}${
        props.isQueued ? " isQueued" : ""
      }`}
      disabled={!clickable}
      title={titleText(clickable, clickMe, props.isQueued)}
    />
  )
})
