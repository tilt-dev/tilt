import {
  Button,
  ButtonProps,
  Checkbox,
  CheckboxProps,
  debounce,
  TextField,
  TextFieldProps,
} from "@material-ui/core"
import React, { useMemo } from "react"
import { AnalyticsAction, incr, Tags } from "./analytics"

// Shared components that implement analytics
// 1. Saves callers from having to implement/test analytics for every interactive
//    component.
// 2. Allows wrappers to cheaply require analytics params.

type InstrumentationProps = {
  analyticsName: string
  analyticsTags?: Tags
}

export function InstrumentedButton(props: ButtonProps & InstrumentationProps) {
  const { analyticsName, analyticsTags, onClick, ...buttonProps } = props
  const instrumentedOnClick: typeof onClick = (e) => {
    incr(analyticsName, {
      action: AnalyticsAction.Click,
      ...(analyticsTags ?? {}),
    })
    if (onClick) {
      onClick(e)
    }
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

// How long to debounce TextField edit events. i.e., only send one edit
// event per this duration. These don't need to be submitted super
// urgently, and we want to be closer to sending one per user intent than
// one per keystroke.
export const textFieldEditDebounceMilliseconds = 5000

export function InstrumentedTextField(
  props: TextFieldProps & InstrumentationProps
) {
  const { analyticsName, analyticsTags, onChange, ...textFieldProps } = props

  // we have to memoize the debounced function so that incrs reuse the same debounce timer
  const debouncedIncr = useMemo(
    () =>
      // debounce so we don't send analytics for every single keypress
      debounce((name: string, tags?: Tags) => {
        incr(name, {
          action: AnalyticsAction.Edit,
          ...(tags ?? {}),
        })
      }, textFieldEditDebounceMilliseconds),
    []
  )

  const instrumentedOnChange: typeof onChange = (e) => {
    debouncedIncr(analyticsName, analyticsTags)
    if (onChange) {
      onChange(e)
    }
  }

  return <TextField onChange={instrumentedOnChange} {...textFieldProps} />
}

export function InstrumentedCheckbox(
  props: CheckboxProps & InstrumentationProps
) {
  const { analyticsName, analyticsTags, onChange, ...checkboxProps } = props
  const instrumentedOnChange: typeof onChange = (e, checked) => {
    incr(analyticsName, {
      action: AnalyticsAction.Edit,
      ...(analyticsTags ?? {}),
    })
    if (onChange) {
      onChange(e, checked)
    }
  }

  return <Checkbox onChange={instrumentedOnChange} {...checkboxProps} />
}
