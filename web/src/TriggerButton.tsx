import React from "react"
import styled from "styled-components"
import { ReactComponent as TriggerButtonManualSvg } from "./assets/svg/trigger-button-manual.svg"
import { ReactComponent as TriggerButtonSvg } from "./assets/svg/trigger-button.svg"
import { InstrumentedButton } from "./instrumentedComponents"
import { AnimDuration, Color, mixinResetButtonStyle } from "./style-helpers"
import { TriggerMode } from "./types"

export let TriggerButtonRoot = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  display: flex;
  align-items: center;
  justify-content: center;

  &.is-disabled {
    pointer-events: none;
  }
  &.is-disabled:active svg {
    transform: scale(1);
  }
  & .fillStd {
    transition: fill ${AnimDuration.default} ease;
    fill: ${Color.grayLight};
  }
  &:hover .fillStd {
    fill: ${Color.white};
  }
  & > svg {
    transition: transform ${AnimDuration.short} linear;
  }
  &:active > svg {
    transform: scale(1.2);
  }
  &.is-building > svg {
    animation: spin 1s linear infinite;
  }
  &.is-queued > svg {
    animation: spin 1s linear infinite;
  }
`

export const TriggerButtonTooltip = {
  AlreadyQueued: "Cannot trigger update. Resource is already queued!",
  NeedsManualTrigger: "Trigger an update",
  UpdateInProgOrPending: "Cannot trigger update. Resource is already updating!",
  ClickToForce: "Force an update",
}

type TriggerButtonProps = {
  isBuilding: boolean
  hasBuilt: boolean
  triggerMode: TriggerMode
  hasPendingChanges: boolean
  isQueued: boolean
  resourceName: string
}

const titleText = (
  disabled: boolean,
  shouldbeClicked: boolean,
  isQueued: boolean
): string => {
  if (isQueued) {
    return TriggerButtonTooltip.AlreadyQueued
  } else if (disabled) {
    return TriggerButtonTooltip.UpdateInProgOrPending
  } else if (shouldbeClicked) {
    return TriggerButtonTooltip.NeedsManualTrigger
  } else {
    return TriggerButtonTooltip.ClickToForce
  }
}

function TriggerButton(props: TriggerButtonProps) {
  let isManualTriggerMode = props.triggerMode !== TriggerMode.TriggerModeAuto
  let isManualTriggerIncludingInitial =
    props.triggerMode === TriggerMode.TriggerModeManual

  // trigger button will only look actionable if there isn't any pending / active build
  let disabled =
    props.isQueued || // already queued for manual run
    props.isBuilding // currently building
  // !(!isManualTriggerIncludingInitial && !props.hasBuilt) || // waiting to perform its initial build
  // !(props.hasPendingChanges && !isManualTriggerMode) // waiting to perform an auto-triggered build in response to a change

  let shouldBeClicked = false
  if (!disabled) {
    if (props.hasPendingChanges && isManualTriggerMode) {
      shouldBeClicked = true
    } else if (!props.hasBuilt && isManualTriggerIncludingInitial) {
      shouldBeClicked = true
    }
  }

  function triggerUpdate(name: string) {
    let url = `//${window.location.host}/api/trigger`

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

  // Add padding to center the icon better.
  let classes = []
  if (disabled) {
    classes.push("is-disabled")
  }
  if (props.isQueued) {
    classes.push("is-queued")
  }
  if (props.isBuilding) {
    classes.push("is-building")
  }
  return (
    <TriggerButtonRoot
      onClick={() => triggerUpdate(props.resourceName)}
      className={classes.join(" ")}
      title={titleText(disabled, shouldBeClicked, props.isQueued)}
      analyticsName={"ui.web.triggerResource"}
    >
      {shouldBeClicked ? <TriggerButtonManualSvg /> : <TriggerButtonSvg />}
    </TriggerButtonRoot>
  )
}

export default React.memo(TriggerButton)
