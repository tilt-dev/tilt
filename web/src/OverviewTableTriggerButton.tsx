import React from "react"
import styled from "styled-components"
import { Tags } from "./analytics"
import { ReactComponent as TriggerButtonSvg } from "./assets/svg/trigger-button.svg"
import { InstrumentedButton } from "./instrumentedComponents"
import { AnimDuration, Color, mixinResetButtonStyle } from "./style-helpers"
import { triggerTooltip, triggerUpdate } from "./trigger"
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

  &.is-disabled {
    cursor: not-allowed;
  }
  &.is-disabled:hover .fillStd {
    fill: ${Color.grayLight};
  }
  &.is-disabled:active svg {
    transform: scale(1);
  }
  &.is-emphasized .fillStd {
    fill: ${Color.blue};
  }
`

type TriggerButtonProps = {
  isBuilding: boolean
  hasBuilt: boolean
  triggerMode: TriggerMode
  hasPendingChanges: boolean
  isQueued: boolean
  resourceName: string
  analyticsTags: Tags
  onTrigger: () => void
}

function OverviewTableTriggerButton(props: TriggerButtonProps) {
  let isManual =
    props.triggerMode === TriggerMode.TriggerModeManual ||
    props.triggerMode === TriggerMode.TriggerModeManualWithAutoInit
  let isAutoInit =
    props.triggerMode === TriggerMode.TriggerModeAuto ||
    props.triggerMode === TriggerMode.TriggerModeManualWithAutoInit

  // clickable (i.e. trigger button will appear) if it doesn't already have some kind of pending / active build
  let clickable =
    !props.isQueued && // already queued for manual run
    !props.isBuilding && // currently building
    !(isAutoInit && !props.hasBuilt) // waiting to perform its initial build

  let isEmphasized = false
  if (clickable) {
    if (props.hasPendingChanges && isManual) {
      isEmphasized = true
    } else if (!props.hasBuilt && !isAutoInit) {
      isEmphasized = true
    }
  }

  let classes = []
  if (!clickable) {
    classes.push("is-disabled")
  }
  if (props.isBuilding) {
    classes.push("is-building")
  }
  if (isEmphasized) {
    classes.push("is-emphasized")
  }
  return (
    <TriggerButtonRoot
      aria-disabled={!clickable}
      onClick={() => triggerUpdate(props.resourceName)}
      className={classes.join(" ")}
      title={triggerTooltip(clickable, isEmphasized, props.isQueued)}
      analyticsName={"ui.web.triggerResource"}
      analyticsTags={props.analyticsTags}
    >
      <TriggerButtonSvg />
    </TriggerButtonRoot>
  )
}

export default React.memo(OverviewTableTriggerButton)
