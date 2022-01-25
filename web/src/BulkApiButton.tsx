import { ButtonClassKey, ButtonGroup, ButtonProps } from "@material-ui/core"
import { ClassNameMap } from "@material-ui/styles"
import React, { useLayoutEffect, useState } from "react"
import styled from "styled-components"
import { AnalyticsType, Tags } from "./analytics"
import {
  ApiButtonToggleState,
  ApiButtonType,
  confirmingButtonGroupBorderMixin,
  confirmingButtonStateMixin,
  UIBUTTON_TOGGLE_INPUT_NAME,
  updateButtonStatus,
} from "./ApiButton"
import { ReactComponent as CloseSvg } from "./assets/svg/close.svg"
import { useHudErrorContext } from "./HudErrorContext"
import { InstrumentedButton } from "./instrumentedComponents"
import { BulkAction } from "./OverviewTableBulkActions"
import { AnimDuration, Color, Font, FontSize, SizeUnit } from "./style-helpers"
import { UIButton } from "./types"

/**
 * The BulkApiButton is used to update multiple UIButtons with a single
 * user action. It follows similar patterns as the core ApiButton component,
 * but most of the data it receives, and its styling, is different.
 * The BulkApiButton supports toggle and non-toggle buttons that may require
 * confirmation.
 *
 * In the future, it may need to be expanded to share more of the UIButton
 * options (like specifying an icon svg or having a form with inputs), or
 * it may need to support non-UIButton bulk actions.
 */

// Types
type BulkApiButtonProps = ButtonProps & {
  bulkAction: BulkAction
  buttonText: string
  className?: string
  onClickCallback?: () => void
  requiresConfirmation: boolean
  targetToggleState?: ApiButtonToggleState
  uiButtons: UIButton[]
}

type BulkApiButtonElementProps = ButtonProps & {
  text: string
  confirming: boolean
  disabled: boolean
  analyticsTags: Tags
  analyticsName: string
}

// Styles
const BulkButtonElementRoot = styled(InstrumentedButton)`
  border: 1px solid ${Color.grayLight};
  border-radius: 4px;
  background-color: ${Color.grayLighter};
  color: ${Color.white};
  font-family: ${Font.monospace};
  font-size: ${FontSize.small};
  padding: 0 ${SizeUnit(1 / 4)};
  text-transform: capitalize;
  transition: color ${AnimDuration.default} ease,
    border ${AnimDuration.default} ease;

  &:hover,
  &:active,
  &:focus {
    background-color: ${Color.grayLighter};
    color: ${Color.blue};
  }

  &.Mui-disabled {
    border-color: ${Color.grayLight};
    color: ${Color.gray6};
  }

  /* Use shared styles with ApiButton */
  ${confirmingButtonStateMixin}
  ${confirmingButtonGroupBorderMixin}
`

const BulkButtonGroup = styled(ButtonGroup)<{ disabled?: boolean }>`
  ${(props) =>
    props.disabled &&
    `
    cursor: not-allowed;
  `}

  /* Adjust the border radius of buttons to achieve the "rounded bar" look */
  &.firstButtonGroupInRow {
    ${BulkButtonElementRoot} {
      border-top-right-radius: 0;
      border-bottom-right-radius: 0;
      border-right: 0;
    }
  }

  &.middleButtonGroupInRow {
    ${BulkButtonElementRoot} {
      border-radius: 0;
      border-right: 0;
    }
  }

  &.lastButtonGroupInRow {
    ${BulkButtonElementRoot} {
      border-top-left-radius: 0;
      border-bottom-left-radius: 0;
    }
  }
`

// Helpers
export function canButtonBeToggled(
  uiButton: UIButton,
  targetToggleState?: ApiButtonToggleState
) {
  const toggleInput = uiButton.spec?.inputs?.find(
    (input) => input.name === UIBUTTON_TOGGLE_INPUT_NAME
  )

  if (!toggleInput) {
    return false
  }

  if (!targetToggleState) {
    return true
  }

  const toggleValue = toggleInput.hidden?.value

  // A button can be toggled if it's state doesn't match the target state
  return toggleValue !== undefined && toggleValue !== targetToggleState
}

/**
 * A bulk button can be toggled if some UIButtons have values that don't
 * match the target toggle state.
 * ex: some buttons are off and target toggle is on => bulk button can be toggled
 * ex: all buttons are on and target toggle is on   => bulk button cannot be toggled
 * ex: all buttons are not toggle buttons           => bulk button cannot be toggled
 */
export function canBulkButtonBeToggled(
  uiButtons: UIButton[],
  targetToggleState: ApiButtonToggleState
) {
  const individualButtonsCanBeToggled = uiButtons.map((b) =>
    canButtonBeToggled(b, targetToggleState)
  )

  return individualButtonsCanBeToggled.some(
    (canBeToggled) => canBeToggled === true
  )
}

function isBulkButtonDisabled(
  uiButtons: UIButton[],
  targetToggleState?: ApiButtonToggleState
) {
  // Bulk button is disabled if there are no UIButtons to trigger
  if (uiButtons.length === 0) {
    return true
  }

  // If there's a target toggle state, calculate whether the bulk button
  // is disabled based on the toggle values of all UIButtons
  if (targetToggleState) {
    const isDisabled = !canBulkButtonBeToggled(uiButtons, targetToggleState)
    return isDisabled
  }

  return false
}

async function bulkUpdateButtonStatus(uiButtons: UIButton[]) {
  try {
    await Promise.all(uiButtons.map((button) => updateButtonStatus(button, {})))
    // console.log("all requests have finished")
  } catch (err) {
    // console.log("yikes, there's been an error")
    throw err
  }
}

function BulkSubmitButton(props: BulkApiButtonElementProps) {
  const {
    analyticsName,
    analyticsTags,
    confirming,
    disabled,
    onClick,
    text,
    ...buttonProps
  } = props

  // Determine display text and accessible button label based on confirmation state
  const displayButtonText = confirming ? "Confirm" : text
  const ariaLabel = confirming ? `Confirm ${text}` : `Trigger ${text}`

  const tags = { ...analyticsTags }
  if (confirming) {
    tags.confirm = "true"
  }

  const isConfirmingClass = confirming ? "confirming leftButtonInGroup" : ""
  const classes: Partial<ClassNameMap<ButtonClassKey>> = {
    root: isConfirmingClass,
  }

  return (
    <BulkButtonElementRoot
      analyticsName={analyticsName}
      analyticsTags={tags}
      aria-label={ariaLabel}
      classes={classes}
      disabled={disabled}
      onClick={onClick}
      {...buttonProps}
    >
      {displayButtonText}
    </BulkButtonElementRoot>
  )
}

function BulkCancelButton(props: BulkApiButtonElementProps) {
  const {
    analyticsName,
    analyticsTags,
    confirming,
    onClick,
    text,
    ...buttonProps
  } = props

  // Don't display the cancel confirmation button if the button
  // group's state isn't confirming
  if (!confirming) {
    return null
  }

  const classes: Partial<ClassNameMap<ButtonClassKey>> = {
    root: "confirming rightButtonInGroup",
  }

  return (
    <BulkButtonElementRoot
      analyticsName={analyticsName}
      aria-label={`Cancel ${text}`}
      analyticsTags={{ confirm: "false", ...analyticsTags }}
      classes={classes}
      onClick={onClick}
      {...buttonProps}
    >
      <CloseSvg role="presentation" />
    </BulkButtonElementRoot>
  )
}

export function BulkApiButton(props: BulkApiButtonProps) {
  const {
    bulkAction,
    buttonText,
    className,
    targetToggleState,
    requiresConfirmation,
    onClickCallback,
    uiButtons,
    ...buttonProps
  } = props

  const { setError } = useHudErrorContext()

  const [loading, setLoading] = useState(false)
  const [confirming, setConfirming] = useState(false)

  const analyticsTags: Tags = {
    component: ApiButtonType.Global,
    type: AnalyticsType.Grid,
    bulkCount: String(uiButtons.length),
    bulkAction,
  }

  if (targetToggleState) {
    // The `toggleValue` reflects the value of the buttons
    // when they are clicked, not their updated values
    analyticsTags.toggleValue =
      targetToggleState === ApiButtonToggleState.On
        ? ApiButtonToggleState.Off
        : ApiButtonToggleState.On
  }

  const bulkActionDisabled = isBulkButtonDisabled(uiButtons, targetToggleState)
  const disabled = loading || bulkActionDisabled || false
  const buttonGroupClassName = `${className} ${
    disabled ? "isDisabled" : "isEnabled"
  }`

  // If the bulk action isn't available while the bulk button
  // is in a confirmation state, reset the confirmation state
  useLayoutEffect(() => {
    if (bulkActionDisabled && confirming) {
      setConfirming(false)
    }
  }, [bulkActionDisabled, confirming])

  const onClick = async () => {
    if (requiresConfirmation && !confirming) {
      setConfirming(true)
      return
    }

    if (confirming) {
      setConfirming(false)
    }

    setLoading(true)

    try {
      // If there's a target toggle state, filter out buttons that
      // already have that toggle state. If they're not filtered out
      // updating them will toggle them to an unintended state.
      const buttonsToUpdate = uiButtons.filter((button) =>
        canButtonBeToggled(button, targetToggleState)
      )
      await bulkUpdateButtonStatus(buttonsToUpdate)
    } catch (err) {
      setError(`Error triggering ${bulkAction} action: ${err}`)
      return
    } finally {
      setLoading(false)

      if (onClickCallback) {
        // console.log("finally, callback!")
        onClickCallback()
      }
    }
  }

  return (
    <BulkButtonGroup
      className={buttonGroupClassName}
      disableRipple={true}
      aria-label={buttonText}
      disabled={disabled}
    >
      <BulkSubmitButton
        analyticsName="ui.web.bulkButton"
        analyticsTags={analyticsTags}
        confirming={confirming}
        disabled={disabled}
        onClick={onClick}
        text={buttonText}
        {...buttonProps}
      ></BulkSubmitButton>
      <BulkCancelButton
        analyticsName="ui.web.bulkButton"
        analyticsTags={analyticsTags}
        confirming={confirming}
        disabled={disabled}
        onClick={() => setConfirming(false)}
        text={buttonText}
        {...buttonProps}
      />
    </BulkButtonGroup>
  )
}
