import React, {PureComponent} from "react"
import {TriggerMode} from "./types"
import "./SidebarTriggerButton.scss"

type SidebarTriggerButtonProps = {
  resourceName: string
  isBuilding: boolean
  hasBuilt: boolean
  triggerMode: TriggerMode
  isSelected: boolean
  hasPendingChanges: boolean
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

export default class SidebarTriggerButton extends PureComponent<
  SidebarTriggerButtonProps
> {

  render() {
    let props = this.props
    let isManualTriggerMode =
        props.triggerMode === TriggerMode.TriggerModeManual

    // isReady (i.e. trigger button will appear) if:
    // 1. resource not currently building, AND
    // 2. resource has built at least once (i.e. we're not waiting for the first build), AND
    //    ^ this will need to change with TRIGGER_MODE_MANUAL_NO_INITIAL
    // 3. resource doesn't have a pending build (i.e. no pending changes, OR pending changes but it's a
    //    manual resource)
    // TODO: don't show trigger button if a manual resource has been queued for build (currently
    //   have no way to detect this)
    let isReady = !props.isBuilding && props.hasBuilt && (!props.hasPendingChanges || isManualTriggerMode)
    let isDirty = props.hasPendingChanges && isManualTriggerMode

    console.log(props)
    console.log("is ready?", isReady)

    return (
      <button
        onClick={() => triggerUpdate(props.resourceName)}
        className={`SidebarTriggerButton ${props.isSelected ? "isSelected" : ""}
          ${isReady ? " isReady": ""}${isDirty ? " isDirty" : ""}`}
      />
    )
  }
}

// ${props.isBuilding || !props.hasBuilt || (props.hasPendingChanges && !isManualTriggerMode) ? "": "isReady"}
