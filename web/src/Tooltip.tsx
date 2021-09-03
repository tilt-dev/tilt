import { makeStyles } from "@material-ui/core/styles"
import Tooltip, { TooltipProps } from "@material-ui/core/Tooltip"
import React from "react"
import { useStorageState } from "react-storage-hooks"
import styled from "styled-components"
import { ReactComponent as InfoSvg } from "./assets/svg/info.svg"
import { InstrumentedButton } from "./instrumentedComponents"
import {
  Color,
  ColorRGBA,
  Font,
  FontSize,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"

const INFO_TOOLTIP_LEAVE_DELAY = 500

let useStyles = makeStyles((theme) => ({
  arrow: {
    color: Color.grayLightest,
    "&::before": {
      border: `1px solid ${Color.grayLight}`,
    },
  },
  tooltip: {
    backgroundColor: Color.grayLightest,
    fontFamily: Font.sansSerif,
    fontSize: FontSize.smallest,
    fontWeight: 400,
    color: Color.grayDark,
    padding: SizeUnit(0.25),
    border: `1px solid ${Color.grayLight}`,
  },
  popper: {
    filter: "drop-shadow(0px 4px 4px rgba(0, 0, 0, 0.25))",
  },
}))

export default function TiltTooltip(props: TooltipProps) {
  const classes = useStyles()

  return (
    <Tooltip
      arrow
      placement="top-start"
      classes={classes}
      role="tooltip"
      {...props}
    />
  )
}

const InfoIcon = styled(InfoSvg)`
  cursor: pointer;
  margin: ${SizeUnit(0.25)};

  &.shadow {
    border-radius: 50%;
    box-shadow: 0px 0px 5px 2px ${ColorRGBA(Color.grayDark, 0.6)};
  }

  .fillStd {
    fill: ${Color.gray6};
  }
`
const DismissButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};

  width: 100%;

  .MuiButton-label {
    font-size: ${FontSize.smallester};
    font-style: italic;
    color: ${Color.grayLight};
    text-align: right;
    display: block;
  }
`

export interface InfoTooltipProps {
  dismissId?: string // If set, this tooltip will be dismissable, keyed by this string
  displayShadow?: boolean
  idForIcon?: string // Use to semantically associate the tooltip with another element through `aria-describedby` or `aria-labelledby`
}

export function TiltInfoTooltip(
  props: Omit<TooltipProps, "children"> & InfoTooltipProps
) {
  const { displayShadow, idForIcon, title, dismissId, ...tooltipProps } = props
  const shadowClass = displayShadow ? "shadow" : ""

  // bug: if multiple tooltips are on the same page with the same key,
  // "dismiss" will only affect the tooltip clicked, until next refresh
  // https://app.clubhouse.io/windmill/story/12654/modifying-usepersistentstate-state-doesn-t-update-other-components-in-the-same-page-using-the-same-key
  const [dismissed, setDismissed] = useStorageState(
    localStorage,
    `tooltip-dismissed-${props.dismissId}`,
    false
  )
  if (dismissed) {
    return null
  }

  let content = title

  if (dismissId !== undefined) {
    content = (
      <>
        <div>{title}</div>
        <DismissButton
          size="small"
          analyticsName={"ui.web.dismissTooltip"}
          analyticsTags={{ tooltip: props.dismissId }}
          onClick={() => setDismissed(true)}
        >
          Don't show this tip
        </DismissButton>
      </>
    )
  }

  return (
    <TiltTooltip
      interactive
      leaveDelay={INFO_TOOLTIP_LEAVE_DELAY}
      title={content}
      {...tooltipProps}
    >
      <InfoIcon id={idForIcon} height={16} width={16} className={shadowClass} />
    </TiltTooltip>
  )
}
