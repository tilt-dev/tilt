import React, { PureComponent } from "react"
import { TriggerMode } from "./types"
import "./SidebarTriggerButton.scss"

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
    body: JSON.stringify({ manifest_names: [name] }),
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
    return "Cannot trigger an update if resource is already queued for build."
  } else if (!clickable) {
    return "Cannot trigger an update while resource is building or build is pending."
  } else if (clickMe) {
    return "This manual resource has pending file changes; click to trigger an update."
  } else {
    return "Force a rebuild/update for this resource."
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
        <button
          className={`SidebarTriggerButton ${
            props.isSelected ? "isSelected" : ""
          }`}
          disabled
          title={"Cannot trigger an update to the Tiltfile."}
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
      <button
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
