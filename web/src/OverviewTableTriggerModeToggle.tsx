import React from "react"
import styled from "styled-components"
import { ReactComponent as TriggerModeButtonSvg } from "./assets/svg/trigger-mode-button.svg"
import { InstrumentedButton } from "./instrumentedComponents"
import { AnimDuration, Color, mixinResetButtonStyle } from "./style-helpers"
import { TriggerMode } from "./types"

let StyledTriggerModeToggle = styled(InstrumentedButton)`
  ${mixinResetButtonStyle}
  display: flex;
  align-items: center;

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
  resourceName: string
  triggerMode: TriggerMode
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

export default function OverviewTableTriggerModeToggle(
  props: TriggerModeToggleProps
) {
  let isManualTriggerMode =
    props.triggerMode == TriggerMode.TriggerModeManualWithAutoInit ||
    props.triggerMode == TriggerMode.TriggerModeManual
  let desiredMode = isManualTriggerMode
    ? // if this manifest WAS Manual_NoInit and hadn't built yet, no guarantee that user wants it to initially build now, but seems like an OK guess
      TriggerMode.TriggerModeAuto
    : // Either manifest was Auto_AutoInit and has already built and the fact that it's now NoInit doesn't make a diff, or was Auto_NoInit in which case we want to preserve the NoInit behavior
      TriggerMode.TriggerModeManual
  let onClick = (e: any) => {
    toggleTriggerMode(props.resourceName, desiredMode)
  }

  return (
    <StyledTriggerModeToggle
      className={isManualTriggerMode ? "is-manual" : ""}
      onClick={onClick}
      title={titleText(isManualTriggerMode)}
      analyticsName="ui.web.toggleTriggerMode"
      analyticsTags={{ toMode: desiredMode.toString() }}
    >
      <TriggerModeButtonSvg />
    </StyledTriggerModeToggle>
  )
}
