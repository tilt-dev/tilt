import React from "react"
import { MemoryRouter } from "react-router"
import styled from "styled-components"
import { ApiButton } from "./ApiButton"
import { OverviewButtonMixin } from "./OverviewButton"
import { TiltSnackbarProvider } from "./Snackbar"
import { oneUIButton, textFieldForUIButton } from "./testdata"
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
