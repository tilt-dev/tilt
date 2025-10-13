// web/src/ApiButtonInputModal.tsx
import React, { useState } from "react"
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  IconButton,
} from "@material-ui/core"
import { Close as CloseIcon } from "@material-ui/icons"
import styled from "styled-components"
import { ApiButtonForm } from "./ApiButton"
import { Color, Font, FontSize } from "./style-helpers"
import { UIButton } from "./types"

// Styled components for better UI
const StyledDialog = styled(Dialog)`
  .MuiDialog-paper {
    min-width: 480px;
    max-width: 600px;
  }
`

const StyledDialogTitle = styled(DialogTitle)`
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-family: ${Font.monospace};
  font-size: ${FontSize.default};
  background-color: ${Color.grayLightest};
  border-bottom: 1px solid ${Color.gray50};

  h2 {
    margin: 0;
    font-weight: normal;
    color: ${Color.gray10};
  }
`

const StyledDialogContent = styled(DialogContent)`
  padding: 24px;
  background-color: white;
`

const StyledDialogActions = styled(DialogActions)`
  padding: 16px 24px;
  background-color: ${Color.grayLightest};
  border-top: 1px solid ${Color.gray50};
  gap: 12px;
`

const CancelButton = styled(Button)`
  color: ${Color.gray40};
  border: 1px solid ${Color.gray50};

  &:hover {
    background-color: ${Color.grayLightest};
    border-color: ${Color.gray40};
  }
`

const ConfirmButton = styled(Button)`
  background-color: ${Color.green};
  color: white;

  &:hover {
    background-color: ${Color.greenLight};
  }

  &:disabled {
    background-color: ${Color.gray50};
    color: ${Color.gray40};
  }
`

export interface ApiButtonInputModalProps {
  open: boolean
  onClose: () => void
  onConfirm: (values: { [name: string]: any }) => void
  uiButton: UIButton
  initialValues: { [name: string]: any }
}

export function ApiButtonInputModal(props: ApiButtonInputModalProps) {
  const [inputValues, setInputValues] = useState(props.initialValues)

  const setInputValue = (name: string, value: any) => {
    setInputValues({ ...inputValues, [name]: value })
  }

  const getInputValue = (name: string) => inputValues[name]

  const handleConfirm = () => {
    props.onConfirm(inputValues)
    props.onClose()
  }

  const buttonText = props.uiButton.spec?.text || "Button"
  const visibleInputs =
    props.uiButton.spec?.inputs?.filter((input) => !input.hidden) || []

  return (
    <StyledDialog
      open={props.open}
      onClose={props.onClose}
      maxWidth="md"
      fullWidth
      aria-labelledby="input-modal-title"
    >
      <StyledDialogTitle id="input-modal-title">
        <span>Configure {buttonText}</span>
        <IconButton
          aria-label="Close dialog"
          onClick={props.onClose}
          size="small"
        >
          <CloseIcon />
        </IconButton>
      </StyledDialogTitle>

      <StyledDialogContent>
        {visibleInputs.length > 0 ? (
          <>
            <p
              style={{
                margin: "0 0 20px 0",
                color: Color.gray40,
                fontSize: FontSize.small,
                fontFamily: Font.monospace,
              }}
            >
              Review and modify the input values, then confirm to execute the
              action.
            </p>
            <ApiButtonForm
              uiButton={props.uiButton}
              setInputValue={setInputValue}
              getInputValue={getInputValue}
            />
          </>
        ) : (
          <p
            style={{
              margin: 0,
              color: Color.gray40,
              fontSize: FontSize.small,
              fontFamily: Font.monospace,
            }}
          >
            Are you sure you want to execute "{buttonText}"?
          </p>
        )}
      </StyledDialogContent>

      <StyledDialogActions>
        <CancelButton onClick={props.onClose} variant="outlined">
          Cancel
        </CancelButton>
        <ConfirmButton
          onClick={handleConfirm}
          variant="contained"
          color="primary"
        >
          Confirm & Execute
        </ConfirmButton>
      </StyledDialogActions>
    </StyledDialog>
  )
}
