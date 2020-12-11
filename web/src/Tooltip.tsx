import { makeStyles } from "@material-ui/core/styles"
import Tooltip from "@material-ui/core/Tooltip"
import React from "react"
import { Color, Font, FontSize, SizeUnit } from "./style-helpers"

type TooltipProps = {
  title: string
  children: React.ReactElement
  open?: boolean // Useful for keeping the tooltip open in storybook
}

export default function TiltTooltip(props: TooltipProps) {
  const classes = makeStyles((theme) => ({
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
  }))()

  return <Tooltip arrow placement="top-end" classes={classes} {...props} />
}
