import React from "react"
import styled from "styled-components"
import { ReactComponent as TriggerButtonSvg } from "./assets/svg/trigger-button.svg"
import { InstrumentedButton } from "./instrumentedComponents"
import { AnimDuration, Color, mixinResetButtonStyle } from "./style-helpers"
import { TriggerMode } from "./types"

export let TriggerButtonRoot = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  display: flex;
  align-items: center;
  justify-content: center;

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

  // DISABLED BUTTON
  &.is-disabled {
    cursor: not-allowed;
  }
  &.is-disabled:hover .fillStd {
    fill: ${Color.grayLight};
  }
  &.is-disabled:active svg {
    transform: scale(1);
  }

  // SHOULD BE CLICKED
  &.shouldBeClicked .fillStd {
    fill: ${Color.blue};
  }
`

export const TriggerButtonTooltip = {
  AlreadyQueued: "Resource is already queued!",
  NeedsManualTrigger: "Trigger an update",
  UpdateInProgOrPending: "Resource is already updating!",
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
  isDisabled: boolean,
  shouldbeClicked: boolean,
  isQueued: boolean
): string => {
  if (isQueued) {
    return TriggerButtonTooltip.AlreadyQueued
  } else if (isDisabled) {
    return TriggerButtonTooltip.UpdateInProgOrPending
  } else if (shouldbeClicked) {
    return TriggerButtonTooltip.NeedsManualTrigger
  } else {
    return TriggerButtonTooltip.ClickToForce
  }
}

function OverviewTableTriggerButton(props: TriggerButtonProps) {
  let isManualTriggerMode = props.triggerMode !== TriggerMode.TriggerModeAuto
  let isManualTriggerIncludingInitial =
    props.triggerMode === TriggerMode.TriggerModeManual

  // trigger button will only look actionable if there isn't any pending / active build
  let isDisabled =
    props.isQueued || // already queued for manual run
    props.isBuilding || // currently building
    (!isManualTriggerIncludingInitial && !props.hasBuilt) || // waiting to perform its initial build
    (props.hasPendingChanges && !isManualTriggerMode) // waiting to perform an auto-triggered build in response to a change

  let shouldBeClicked = false
  if (!isDisabled) {
    if (props.hasPendingChanges && isManualTriggerMode) {
      shouldBeClicked = true
    } else if (!props.hasBuilt && isManualTriggerIncludingInitial) {
      shouldBeClicked = true
    }
  }

  function triggerUpdate(name: string) {
    let url = `/api/trigger`

    !isDisabled &&
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

  let classes = []
  if (isDisabled) {
    classes.push("is-disabled")
  }
  if (props.isQueued) {
    classes.push("is-queued")
  }
  if (props.isBuilding) {
    classes.push("is-building")
  }
  if (shouldBeClicked) {
    classes.push("shouldBeClicked")
  }
  return (
    <TriggerButtonRoot
      aria-disabled={isDisabled}
      onClick={() => triggerUpdate(props.resourceName)}
      className={classes.join(" ")}
      title={titleText(isDisabled, shouldBeClicked, props.isQueued)}
      analyticsName={"ui.web.triggerResource"}
    >
      <TriggerButtonSvg />
    </TriggerButtonRoot>
  )
}

export default React.memo(OverviewTableTriggerButton)
