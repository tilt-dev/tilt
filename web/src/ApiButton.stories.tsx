import React from "react"
import { MemoryRouter } from "react-router"
import styled from "styled-components"
import { ApiButton } from "./ApiButton"
import { OverviewButtonMixin } from "./OverviewButton"
import { TiltSnackbarProvider } from "./Snackbar"
import {
  oneUIButton,
  textFieldForUIButton,
  boolFieldForUIButton,
} from "./testdata"
import { UIInputSpec } from "./types"

export default {
  title: "New UI/Shared/ApiButton",
  decorators: [
    (Story: any) => (
      <MemoryRouter initialEntries={["/"]}>
        <TiltSnackbarProvider>
          <Story />
        </TiltSnackbarProvider>
      </MemoryRouter>
    ),
  ],
}

const StyledButton = styled(ApiButton)`
  button {
    ${OverviewButtonMixin};
  }
`

export const SimpleButton = () => {
  const button = oneUIButton({})
  return <StyledButton uiButton={button} />
}

export const RequiresConfirmation = () => {
  const button = oneUIButton({ requiresConfirmation: true })
  return <StyledButton uiButton={button} />
}

export const ThreeTextInputs = () => {
  const inputs: UIInputSpec[] = [1, 2, 3].map((i) =>
    textFieldForUIButton(`text${i}`)
  )
  const button = oneUIButton({ inputSpecs: inputs })
  return <StyledButton uiButton={button} />
}

export const TextInputOptions = () => {
  const button = oneUIButton({
    inputSpecs: [
      textFieldForUIButton("text1", undefined, "placeholder"),
      textFieldForUIButton("text2", "default value"),
    ],
  })
  return <StyledButton uiButton={button} />
}

export const ButtonWithModal = () => {
  const button = oneUIButton({
    buttonText: "Deploy with Modal",
    inputSpecs: [
      textFieldForUIButton("environment", "dev", "dev, staging, prod"),
      textFieldForUIButton("replicas", "1", "1-10"),
    ],
  })
  return <StyledButton uiButton={button} />
}

export const ModalWithManyInputs = () => {
  const button = oneUIButton({
    buttonText: "Deploy Complex App",
    inputSpecs: [
      textFieldForUIButton(
        "environment",
        "dev",
        "Environment (dev/staging/prod)"
      ),
      textFieldForUIButton("replicas", "3", "Number of replicas"),
      textFieldForUIButton("version", "latest", "Image version"),
      textFieldForUIButton("namespace", "default", "Kubernetes namespace"),
      boolFieldForUIButton("enable_debug", false),
    ],
  })
  return <StyledButton uiButton={button} />
}

export const ModalWithConfirmation = () => {
  const button = oneUIButton({
    buttonText: "Delete Resources",
    requiresConfirmation: true,
    inputSpecs: [textFieldForUIButton("reason", "", "Reason for deletion")],
  })
  return <StyledButton uiButton={button} />
}
