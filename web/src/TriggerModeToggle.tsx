import React from "react"
import styled from "styled-components"
import { ReactComponent as AutoSvg } from "./assets/svg/auto.svg"
import {
  AnimDuration,
  Color,
  ColorAlpha,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"
import { TriggerMode } from "./types"

let TriggerModeToggleRoot = styled.button`
  width: ${SizeUnit(1)};
  height: ${SizeUnit(1)};

  ${mixinResetButtonStyle}

  .fillStd {
    fill: ${Color.blue};
    transition: fill ${AnimDuration.short} linear;
  }
  .strokeStd {
    stroke: ${Color.blue};
    opacity: ${ColorAlpha.almostOpaque};
    transition: stroke ${AnimDuration.short} linear;
  }
  .autoSvg-toggleOn {
    fill: ${Color.blue};
  }

  &.is-manual {
    .fillStd {
      fill: ${Color.grayLight};
    }
    .strokeStd {
      stroke: ${Color.grayLight};
    }
    .autoSvg-toggleOn {
      fill: none;
    }
    .autoSvg-toggleOff {
      fill: ${Color.grayLight};
    }
  }
`

type TriggerModeToggleProps = {
  triggerMode: TriggerMode
  onModeToggle: (mode: TriggerMode) => void
  // TODO: is set from UI? (bool)
}

export const ToggleTriggerModeTooltip = {
  isManual: "Auto OFF (file changes do not trigger updates)",
  isAuto: "Auto ON (file changes trigger update)",
}

const titleText = (isManual: boolean): string => {
  if (isManual) {
    return ToggleTriggerModeTooltip.isManual
  } else {
    return ToggleTriggerModeTooltip.isAuto
  }
}

function TriggerModeToggle(props: TriggerModeToggleProps) {
  let isManualTriggerMode = props.triggerMode !== TriggerMode.TriggerModeAuto
  let desiredMode = isManualTriggerMode
    ? TriggerMode.TriggerModeAuto
    : TriggerMode.TriggerModeManualAfterInitial
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
    >
      <AutoSvg />
    </TriggerModeToggleRoot>
  )
}

export { TriggerModeToggle, TriggerModeToggleRoot }
