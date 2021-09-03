import React from "react"
import { MemoryRouter } from "react-router"
import styled from "styled-components"
import { ApiButton } from "./ApiButton"
import { makeUIButton, textField } from "./ApiButton.testhelpers"
import { OverviewButtonMixin } from "./OverviewButton"

type UIButton = Proto.v1alpha1UIButton
type UIInputSpec = Proto.v1alpha1UIInputSpec
type UITextInputSpec = Proto.v1alpha1UITextInputSpec
type UIInputStatus = Proto.v1alpha1UIInputStatus

export default {
  title: "New UI/Shared/ApiButton",
  decorators: [
    (Story: any) => (
      <MemoryRouter initialEntries={["/"]}>
        <div style={{ margin: "-1rem" }}>
          <Story />
        </div>
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
  const button = makeUIButton()
  return <StyledButton uiButton={button} />
}

export const ThreeTextInputs = () => {
  const inputs: UIInputSpec[] = [1, 2, 3].map((i) => textField(`text${i}`))
  const button = makeUIButton({ inputSpecs: inputs })
  return <StyledButton uiButton={button} />
}

export const TextInputOptions = () => {
  const button = makeUIButton({
    inputSpecs: [
      textField("text1", undefined, "placeholder"),
      textField("text2", "default value"),
    ],
  })
  return <StyledButton uiButton={button} />
}
