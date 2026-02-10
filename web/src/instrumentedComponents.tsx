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

// Shared components that implement analytics
// 1. Saves callers from having to implement/test analytics for every interactive
//    component.
// 2. Allows wrappers to cheaply require analytics params.

type InstrumentationProps = {}

export const InstrumentedButton = React.forwardRef<
  HTMLButtonElement,
  ButtonProps & InstrumentationProps
>(function InstrumentedButton(props, ref) {
  const { onClick, ...buttonProps } = props
  const instrumentedOnClick: typeof onClick = (e) => {
    if (onClick) {
      onClick(e)
    }
  }

  // TODO(nick): variant="outline" doesn't seem like the right default.
  return (
    <Button
      ref={ref}
      variant="outlined"
      disableRipple={true}
      onClick={instrumentedOnClick}
      {...buttonProps}
    />
  )
})

// How long to debounce TextField edit events. i.e., only send one edit
// event per this duration. These don't need to be submitted super
// urgently, and we want to be closer to sending one per user intent than
// one per keystroke.
export const textFieldEditDebounceMilliseconds = 5000

export function InstrumentedTextField(
  props: TextFieldProps & InstrumentationProps
) {
  const { onChange, ...textFieldProps } = props

  const instrumentedOnChange: typeof onChange = (e) => {
    if (onChange) {
      onChange(e)
    }
  }

  return <TextField onChange={instrumentedOnChange} {...textFieldProps} />
}

export function InstrumentedCheckbox(
  props: CheckboxProps & InstrumentationProps
) {
  const { onChange, ...checkboxProps } = props
  const instrumentedOnChange: typeof onChange = (e, checked) => {
    if (onChange) {
      onChange(e, checked)
    }
  }

  return (
    <Checkbox
      onChange={instrumentedOnChange}
      disableRipple={true}
      {...checkboxProps}
    />
  )
}
