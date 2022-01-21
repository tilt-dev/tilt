import { ButtonProps } from "@material-ui/core"
import React, { useState } from "react"
import {
  ApiButtonRoot,
  ApiButtonToggleState,
  ApiCancelButton,
  ApiSubmitButton,
  UIBUTTON_TOGGLE_INPUT_NAME,
  updateButtonStatus,
} from "./ApiButton"
import { useHudErrorContext } from "./HudErrorContext"
import { BulkAction } from "./OverviewTableBulkActions"
import { UIButton } from "./types"

/**
 * TODO: Add a note here about:
 * - Forking the ApiButton code
 * - Possibilities for future refactoring (possible that other bulk actions may not be `UIButton`s)
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

// Styles

// Helpers
function canButtonBeToggled(
  uiButton: UIButton,
  targetToggleState?: ApiButtonToggleState
) {
  if (!targetToggleState) {
    return true
  }

  const toggleInput = uiButton.spec?.inputs?.find(
    (input) => input.name === UIBUTTON_TOGGLE_INPUT_NAME
  )

  if (!toggleInput) {
    return false
  }

  const toggleValue = toggleInput.hidden?.value

  // A button can be toggled if it's state doesn't match the target state
  return toggleValue !== undefined && toggleValue !== targetToggleState
}

// A bulk button can be toggled if some buttons have values that don't match the target toggle state
// ex: all buttons are not toggle buttons => bulk button cannot be toggled
// ex: all buttons are on and target toggle is on => bulk button cannot be toggled
// ex: some buttons are off and target toggle is on => bulk button can be toggled
function canBulkButtonBeToggled(
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

// TODO: Add unit tests for this...
// Or possibly re-write if it's too confusing
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
  const buttonClicks: Promise<void>[] = uiButtons.map((button) =>
    updateButtonStatus(button, {})
  )

  try {
    await Promise.all(buttonClicks)
  } catch (err) {
    throw err
  }
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

  // TODO: Create analytics tags

  const bulkActionDisabled = isBulkButtonDisabled(uiButtons, targetToggleState)

  const disabled = loading || bulkActionDisabled || false

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
      // updating them will toggle them to a non-target state.
      const buttonsToUpdate = uiButtons.filter((button) =>
        canButtonBeToggled(button, targetToggleState)
      )
      await bulkUpdateButtonStatus(buttonsToUpdate)
    } catch (err) {
      setError(`Error submitting button click: ${err}`)
      return
    } finally {
      if (onClickCallback) {
        onClickCallback()
      }

      setLoading(false)
    }
  }

  return (
    <ApiButtonRoot
      className={className}
      disableRipple={true}
      aria-label={buttonText}
      disabled={disabled}
    >
      <ApiSubmitButton
        analyticsName="ui.web.bulkButton"
        analyticsTags={{}}
        confirming={confirming}
        disabled={disabled}
        onClick={onClick}
        text={buttonText}
        {...buttonProps}
      ></ApiSubmitButton>
      <ApiCancelButton
        analyticsName="ui.web.bulkButton"
        analyticsTags={{}}
        confirming={confirming}
        disabled={disabled}
        onClick={() => setConfirming(false)}
        text={buttonText}
        {...buttonProps}
      />
    </ApiButtonRoot>
  )
}
