import { makeStyles } from "@material-ui/core/styles"
import Tooltip, { TooltipProps } from "@material-ui/core/Tooltip"
import React from "react"
import styled from "styled-components"
import { ReactComponent as InfoSvg } from "./assets/svg/info.svg"
import { Color, ColorRGBA, Font, FontSize, SizeUnit } from "./style-helpers"

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

export interface InfoTooltipProps {
  displayShadow?: boolean
  idForIcon?: string // Use to semantically associate the tooltip with another element through `aria-describedby` or `aria-labelledby`
}

export function TiltInfoTooltip(
  props: Omit<TooltipProps, "children"> & InfoTooltipProps
) {
  const { displayShadow, idForIcon, ...tooltipProps } = props
  const shadowClass = displayShadow ? "shadow" : ""

  return (
    <TiltTooltip
      interactive
      leaveDelay={INFO_TOOLTIP_LEAVE_DELAY}
      {...tooltipProps}
    >
      <InfoIcon id={idForIcon} height={16} width={16} className={shadowClass} />
    </TiltTooltip>
  )
}
