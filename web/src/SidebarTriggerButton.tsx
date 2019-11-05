import React, { PureComponent } from "react"
import { TriggerMode } from "./types"
import "./SidebarTriggerButton.scss"

type SidebarTriggerButtonProps = {
  resourceName: string
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
  isReady: boolean,
  isDirty: boolean,
  isQueued: boolean
): string => {
  if (isQueued) {
    return "Cannot trigger an update if resource is already queued for build."
  } else if (!isReady) {
    return "Cannot trigger an update while resource is building or build is pending."
  } else if (isDirty) {
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
    if (props.resourceName == "(Tiltfile)") {
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

    let isManualTriggerMode =
      props.triggerMode === TriggerMode.TriggerModeManual

    // isReady (i.e. trigger button will appear) if:
    // 1. resource not currently building, AND
    // 2. resource is not queued to build, AND
    // 3. resource has built at least once (i.e. we're not waiting for the first build), AND
    //    ^ this will need to change with TRIGGER_MODE_MANUAL_NO_INITIAL
    // 4. resource doesn't have a pending auto-build (i.e. no pending changes, OR pending
    //    changes but it's a manual resource)
    let isReady =
      !props.isQueued &&
      !props.isBuilding &&
      props.hasBuilt &&
      (!props.hasPendingChanges || isManualTriggerMode)
    let isDirty = props.hasPendingChanges && isManualTriggerMode

    return (
      <button
        onClick={() => {
          triggerUpdate(props.resourceName)
        }}
        className={`SidebarTriggerButton ${props.isSelected ? "isSelected" : ""}
          ${isReady ? " isReady" : ""}${isDirty ? " isDirty" : ""}${
          props.isQueued ? " isQueued" : ""
        }`}
        disabled={!isReady}
        title={titleText(isReady, isDirty, props.isQueued)}
      />
    )
  }
}
