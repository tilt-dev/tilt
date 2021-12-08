// Helpers for triggering updates.

import { TriggerMode } from "./types"

export const TriggerButtonTooltip = {
  AlreadyQueued: "Resource already queued!",
  NeedsManualTrigger: "Trigger update to sync changes",
  UpdateInProgOrPending: "Resource already updating!",
  Default: "Trigger update",
}

export function triggerTooltip(
  isClickable: boolean,
  isEmphasized: boolean,
  isQueued: boolean
): string {
  if (isQueued) {
    return TriggerButtonTooltip.AlreadyQueued
  } else if (!isClickable) {
    return TriggerButtonTooltip.UpdateInProgOrPending
  } else if (isEmphasized) {
    return TriggerButtonTooltip.NeedsManualTrigger
  } else {
    return TriggerButtonTooltip.Default
  }
}

export function triggerUpdate(name: string) {
  let url = `/api/trigger`

  fetch(url, {
    method: "post",
    body: JSON.stringify({
      manifest_names: [name],
      build_reason: 16 /* BuildReasonFlagTriggerWeb */,
    }),
  }).then((response) => {
    if (!response.ok) {
      console.log(response)
    }
  })
}

export function toggleTriggerMode(name: string, mode: TriggerMode) {
  let url = "/api/override/trigger_mode"

  fetch(url, {
    method: "post",
    body: JSON.stringify({
      manifest_names: [name],
      trigger_mode: mode,
    }),
  }).then((response) => {
    if (!response.ok) {
      console.log(response)
    }
  })
}
