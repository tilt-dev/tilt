import React from "react"
import { render, fireEvent, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { ApiButtonInputModal } from "./ApiButtonInputModal"
import {
  oneUIButton,
  textFieldForUIButton,
  boolFieldForUIButton,
} from "./testdata"

describe("ApiButtonInputModal", () => {
  it("renders correctly", () => {
    const uiButton = oneUIButton({
      buttonText: "Test Button",
      inputSpecs: [textFieldForUIButton("test", "default", "placeholder")],
    })

    const { getByText } = render(
      <ApiButtonInputModal
        open={true}
        onClose={() => {}}
        onConfirm={() => {}}
        uiButton={uiButton}
        initialValues={{ test: "default" }}
      />
    )

    expect(getByText("Configure Test Button")).toBeInTheDocument()
    expect(getByText("Confirm & Execute")).toBeInTheDocument()
    expect(getByText("Cancel")).toBeInTheDocument()
  })

  it("calls onConfirm with correct values when confirmed", async () => {
    const mockOnConfirm = jest.fn()
    const uiButton = oneUIButton({
      buttonText: "Deploy App",
      inputSpecs: [
        textFieldForUIButton("environment", "dev", "Environment"),
        boolFieldForUIButton("debug", false),
      ],
    })

    const { getByText, getByLabelText } = render(
      <ApiButtonInputModal
        open={true}
        onClose={() => {}}
        onConfirm={mockOnConfirm}
        uiButton={uiButton}
        initialValues={{ environment: "staging", debug: true }}
      />
    )

    // Verify initial values are displayed
    expect(getByLabelText("environment")).toHaveValue("staging")

    // Click confirm
    userEvent.click(getByText("Confirm & Execute"))

    await waitFor(() => {
      expect(mockOnConfirm).toHaveBeenCalledWith({
        environment: "staging",
        debug: true,
      })
    })
  })

  it("calls onClose when Cancel is clicked", () => {
    const mockOnClose = jest.fn()
    const uiButton = oneUIButton({ buttonText: "Test Button" })

    const { getByText } = render(
      <ApiButtonInputModal
        open={true}
        onClose={mockOnClose}
        onConfirm={() => {}}
        uiButton={uiButton}
        initialValues={{}}
      />
    )

    userEvent.click(getByText("Cancel"))
    expect(mockOnClose).toHaveBeenCalled()
  })

  it("shows confirmation message when no inputs are present", () => {
    const uiButton = oneUIButton({
      buttonText: "Delete All",
      inputSpecs: [], // No inputs
    })

    const { getByText } = render(
      <ApiButtonInputModal
        open={true}
        onClose={() => {}}
        onConfirm={() => {}}
        uiButton={uiButton}
        initialValues={{}}
      />
    )

    expect(
      getByText('Are you sure you want to execute "Delete All"?')
    ).toBeInTheDocument()
  })
})
