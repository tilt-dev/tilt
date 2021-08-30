import {
  Button,
  ButtonProps,
  debounce,
  TextField,
  TextFieldProps,
} from "@material-ui/core"
import React, { useMemo } from "react"
import { AnalyticsAction, incr, Tags } from "./analytics"

// Shared components that implement analytics
// 1. Saves callers from having to implement/test analytics for every interactive
//    component.
// 2. Allows wrappers to cheaply require uses specify analytics params.

type InstrumentationProps = {
  analyticsName: string
  analyticsTags?: Tags
}

export function InstrumentedButton(props: ButtonProps & InstrumentationProps) {
  const { analyticsName, analyticsTags, onClick, ...buttonProps } = { ...props }
  const instrumentedOnClick: typeof onClick = (e) => {
    if (onClick) {
      onClick(e)
    }
    incr(analyticsName, {
      action: AnalyticsAction.Click,
      ...(analyticsTags ?? {}),
    })
  }

  return (
    <Button
      variant="outlined"
      disableRipple={true}
      onClick={instrumentedOnClick}
      {...buttonProps}
    />
  )
}

export function InstrumentedTextField(
  props: TextFieldProps & InstrumentationProps
) {
  const { analyticsName, analyticsTags, onChange, ...textFieldProps } = {
    ...props,
  }

  // we have to memoize the debounced function so that incrs reuse the same debounce timer
  const debouncedIncr = useMemo(
    () =>
      // debounce so we don't send analytics for every single keypress
      debounce(() => {
        incr(analyticsName, {
          action: AnalyticsAction.Edit,
          ...(analyticsTags ?? {}),
        })
      }, 5000),
    [analyticsName, analyticsTags]
  )

  const instrumentedOnChange: typeof onChange = (e) => {
    if (onChange) {
      onChange(e)
    }
    debouncedIncr()
  }

  return <TextField onChange={instrumentedOnChange} {...textFieldProps} />
}
