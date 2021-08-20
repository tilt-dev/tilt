import { Button, ButtonProps } from "@material-ui/core"
import React from "react"
import { AnalyticsAction, incr, Tags } from "./analytics"

export type InstrumentedButtonProps = ButtonProps & {
  analyticsName: string
  analyticsTags?: Tags
}

export function InstrumentedButton(props: InstrumentedButtonProps) {
  const { analyticsName, analyticsTags, onClick, ...buttonProps } = { ...props }
  const tags = { action: AnalyticsAction.Click, ...(analyticsTags ?? {}) }
  const instrumentedOnClick = (
    e: React.MouseEvent<HTMLButtonElement, MouseEvent>
  ) => {
    incr(analyticsName, tags)
    if (onClick) {
      onClick(e)
    }
  }
  return <Button variant="outlined" onClick={instrumentedOnClick} {...buttonProps} />
}
