import React, { useCallback } from "react"
import styled from "styled-components"
import { Tags } from "./analytics"
import { ApiButton } from "./ApiButton"
import { ReactComponent as StartBuildButtonManualSvg } from "./assets/svg/start-build-button-manual.svg"
import { ReactComponent as StartBuildButtonSvg } from "./assets/svg/start-build-button.svg"
import { ReactComponent as StopBuildButtonSvg } from "./assets/svg/stop-build-button.svg"
import { InstrumentedButton } from "./instrumentedComponents"
import TiltTooltip from "./Tooltip"
import { BuildButtonTooltip, buildButtonTooltip } from "./trigger"
import { TriggerMode, UIButton } from "./types"

export type StartBuildButtonProps = {
  isBuilding: boolean
  hasBuilt: boolean
  triggerMode: TriggerMode
  isSelected?: boolean
  hasPendingChanges: boolean
  isQueued: boolean
  onStartBuild: () => void
  analyticsTags: Tags
  className?: string
}

type StopBuildButtonProps = {
  stopBuildButton?: UIButton
}

export type BuildButtonProps = StartBuildButtonProps & StopBuildButtonProps

function BuildButton(props: BuildButtonProps) {
  const { stopBuildButton, ...startBuildButtonProps } = props
  if (props.isBuilding) {
    if (!stopBuildButton) {
      return null
    }
    let classes = [props.className, "stop-button", "is-clickable"]
    if (props.isSelected) {
      classes.push("is-selected")
    }

    return (
      <TiltTooltip title={BuildButtonTooltip.Stop}>
        <BuildButtonCursorWrapper>
          <ApiButton uiButton={stopBuildButton} className={classes.join(" ")}>
            <StopBuildButtonSvg className="icon" />
          </ApiButton>
        </BuildButtonCursorWrapper>
      </TiltTooltip>
    )
  } else {
    const classes = [props.className, "start-button"]
    return (
      <StartBuildButton
        {...startBuildButtonProps}
        className={classes.join(" ")}
      />
    )
  }
}

// A wrapper to receive pointer events so that we get cursor and tooltip when disabled
// https://mui.com/components/tooltips/#disabled-elements
const BuildButtonCursorWrapper = styled.div`
  display: inline-block;
  cursor: not-allowed;
  // keep the button in front of the "in-progress" barber pole animation
  z-index: 1;
  .is-clickable {
    cursor: pointer;
  }
`

function StartBuildButton(props: StartBuildButtonProps) {
  let isManual =
    props.triggerMode === TriggerMode.TriggerModeManual ||
    props.triggerMode === TriggerMode.TriggerModeManualWithAutoInit
  let isAutoInit =
    props.triggerMode === TriggerMode.TriggerModeAuto ||
    props.triggerMode === TriggerMode.TriggerModeManualWithAutoInit

  // clickable (i.e. start build button will appear) if it doesn't already have some kind of pending / active build
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

  let onClick = useCallback(
    (e: React.MouseEvent<HTMLButtonElement>) => {
      // In the sidebar, StartBuildButton is nested in a link,
      // and preventDefault is the standard way to cancel the navigation.
      e.preventDefault()

      // stopPropagation prevents the overview card from opening.
      e.stopPropagation()

      props.onStartBuild()
    },
    [props.onStartBuild]
  )

  let classes = [props.className]
  if (props.isSelected) {
    classes.push("is-selected")
  }
  if (clickable) {
    classes.push("is-clickable")
  }
  if (props.isQueued) {
    classes.push("is-queued")
  }
  if (isManual) {
    classes.push("is-manual")
  }
  if (isEmphasized) {
    classes.push("is-emphasized")
  }
  if (props.isBuilding) {
    classes.push("is-building")
  }
  const tooltip = buildButtonTooltip(clickable, isEmphasized, props.isQueued)
  // Set the tooltip key to the tooltip message so that each message is a different "component" and enterNextDelay
  // applies when the message changes.
  // Otherwise, we often display a flicker of "resource is already queued!" after clicking "start build" before
  // the "stop build" button appears.
  return (
    <TiltTooltip title={tooltip} key={tooltip}>
      <BuildButtonCursorWrapper
        className={clickable ? ".is-clickable" : undefined}
      >
        <InstrumentedButton
          onClick={onClick}
          className={classes.join(" ")}
          disabled={!clickable}
          aria-label={tooltip}
          analyticsName={"ui.web.triggerResource"}
          analyticsTags={props.analyticsTags}
        >
          {isEmphasized ? (
            <StartBuildButtonManualSvg role="presentation" className="icon" />
          ) : (
            <StartBuildButtonSvg role="presentation" className="icon" />
          )}
        </InstrumentedButton>
      </BuildButtonCursorWrapper>
    </TiltTooltip>
  )
}

export default React.memo(BuildButton)
