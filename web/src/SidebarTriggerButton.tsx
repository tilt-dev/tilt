import React, { PureComponent } from "react"
import { TriggerMode } from "./types"
import "./SidebarTriggerButton.scss"
import styled from "styled-components"
import { Height, Width } from "./style-helpers"

let SidebarTriggerButtonStyle = styled.button`
  background-position: center center;
  background-repeat: no-repeat;
  background-color: transparent;
  border: 0 none;
  height: ${Height.sidebarItem}px;
  width: ${Width.sidebarTriggerButton}px;
  display: flex;
  align-items: center;
  justify-content: center;
`

export const TriggerButtonTooltip = {
  AlreadyQueued:
    "Cannot trigger an update if resource is already queued for update.",
  ManualResourcePendingChanges:
    "This manual resource has pending file changes; click to trigger an update.",
  UpdateInProgOrPending:
    "Cannot trigger an update while resource is updating or update is pending.",
  ClickToForce: "Force a rebuild/update for this resource.",
  CannotTriggerTiltfile: "Cannot trigger an update to the Tiltfile.",
}

type SidebarTriggerButtonProps = {
  resourceName: string
  isTiltfile: boolean
  isBuilding: boolean
  hasBuilt: boolean
  triggerMode: TriggerMode
  isSelected: boolean
  hasPendingChanges: boolean
  isQueued: boolean
}

const triggerUpdate = (name: string): void => {
  let url = `//${window.location.host}/api/trigger`

  fetch(url, {
    method: "post",
    body: JSON.stringify({
      manifest_names: [name],
      build_reason: 16 /* BuildReasonFlagTriggerWeb */,
    }),
  }).then(response => {
    if (!response.ok) {
      console.log(response)
    }
  })
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

export default class SidebarTriggerButton extends PureComponent<
  SidebarTriggerButtonProps
> {
  render() {
    let props = this.props
    if (props.isTiltfile) {
      // can't force update the Tiltfile
      return (
        <SidebarTriggerButtonStyle
          className={`SidebarTriggerButton ${
            props.isSelected ? "isSelected" : ""
          }`}
          disabled
          title={TriggerButtonTooltip.CannotTriggerTiltfile}
        />
      )
    }

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

    return (
      <SidebarTriggerButtonStyle
        onClick={() => {
          triggerUpdate(props.resourceName)
        }}
        className={`SidebarTriggerButton ${props.isSelected ? "isSelected" : ""}
          ${clickable ? " clickable" : ""}${clickMe ? " clickMe" : ""}${
          props.isQueued ? " isQueued" : ""
        }`}
        disabled={!clickable}
        title={titleText(clickable, clickMe, props.isQueued)}
      />
    )
  }
}
