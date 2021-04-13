import React from "react"
import styled from "styled-components"
import { ReactComponent as TriggerModeButtonSvg } from "./assets/svg/trigger-mode-button.svg"
import { InstrumentedButton } from "./instrumentedComponents"
import { AnimDuration, Color, mixinResetButtonStyle } from "./style-helpers"
import { TriggerMode } from "./types"

let TriggerModeToggleRoot = styled(InstrumentedButton)`
  ${mixinResetButtonStyle}
  display: flex;
  align-items: center;
  transition: opacity ${AnimDuration.short} linear;
  opacity: 0;

  .u-showTriggerModeOnHover:hover &,
  &:focus {
    opacity: 1;
  }

  .fillStd {
    fill: ${Color.grayDark};
  }
  .strokeStd {
    stroke: ${Color.grayLight};
    transition: stroke ${AnimDuration.short} linear;
  }
  .triggerModeSvg-isManual {
    opacity: 0;
    fill: ${Color.blue};
    transition: opacity ${AnimDuration.default} ease;
  }
  .triggerModeSvg-isAuto {
    fill: ${Color.grayLight};
    transition: opacity ${AnimDuration.default} ease;
  }

  &.is-manual {
    .strokeStd {
      stroke: ${Color.blue};
    }
    .triggerModeSvg-isManual {
      opacity: 1;
    }
    .triggerModeSvg-isAuto {
      opacity: 0;
    }
  }
`

type TriggerModeToggleProps = {
  triggerMode: TriggerMode
  onModeToggle: (mode: TriggerMode) => void
  // TODO: is set from UI? (bool)
}

export const ToggleTriggerModeTooltip = {
  isManual: "File changes do not trigger updates",
  isAuto: "File changes trigger update",
}

const titleText = (isManual: boolean): string => {
  if (isManual) {
    return ToggleTriggerModeTooltip.isManual
  } else {
    return ToggleTriggerModeTooltip.isAuto
  }
}

function TriggerModeToggle(props: TriggerModeToggleProps) {
  let isManualTriggerMode =
    props.triggerMode == TriggerMode.TriggerModeManualWithAutoInit ||
    props.triggerMode == TriggerMode.TriggerModeManual
  let desiredMode = isManualTriggerMode
    ? // if this manifest WAS Manual_NoInit and hadn't built yet, no guarantee that user wants it to initially build now, but seems like an OK guess
      TriggerMode.TriggerModeAuto
    : // Either manifest was Auto_AutoInit and has already built and the fact that it's now NoInit doesn't make a diff, or was Auto_NoInit in which case we want to preserve the NoInit behavior
      TriggerMode.TriggerModeManual
  let onClick = (e: any) => {
    // TriggerModeToggle is nested in a link,
    // and preventDefault is the standard way to cancel the navigation.
    e.preventDefault()

    // stopPropagation prevents the overview card from opening.
    e.stopPropagation()

    props.onModeToggle(desiredMode)
  }

  return (
    <TriggerModeToggleRoot
      className={isManualTriggerMode ? "is-manual" : ""}
      onClick={onClick}
      title={titleText(isManualTriggerMode)}
      analyticsName="ui.web.toggleTriggerMode"
      analyticsTags={{ toMode: desiredMode.toString() }}
    >
      <TriggerModeButtonSvg />
    </TriggerModeToggleRoot>
  )
}

export { TriggerModeToggle, TriggerModeToggleRoot }
