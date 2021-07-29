import { makeStyles } from "@material-ui/core/styles"
import Tooltip, { TooltipProps } from "@material-ui/core/Tooltip"
import React from "react"
import styled from "styled-components"
import { ReactComponent as InfoSvg } from "./assets/svg/info.svg"
import { Color, Font, FontSize, SizeUnit } from "./style-helpers"

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
      placement="top-end"
      classes={classes}
      role="tooltip"
      {...props}
    />
  )
}

const InfoIcon = styled(InfoSvg)`
  padding: ${SizeUnit(0.25)};
  cursor: pointer;

  .fillStd {
    fill: ${Color.gray6};
  }
`

interface InfoTooltipProps {
  idForIcon?: string // Use to semantically associate the tooltip with another element through `aria-describedby` or `aria-labelledby`
}

export function TiltInfoTooltip(
  props: Omit<TooltipProps, "children"> & InfoTooltipProps
) {
  return (
    <TiltTooltip interactive leaveDelay={INFO_TOOLTIP_LEAVE_DELAY} {...props}>
      <InfoIcon id={props.idForIcon ?? ""} height={16} width={16} />
    </TiltTooltip>
  )
}
