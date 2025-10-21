import {
  render,
  RenderOptions,
  RenderResult,
  screen,
  waitFor,
} from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import fetchMock from "fetch-mock"
import { SnackbarProvider } from "notistack"
import React, { PropsWithChildren } from "react"
import { MemoryRouter } from "react-router"
import {
  ApiButton,
  ApiButtonType,
  buttonsByComponent,
  ButtonSet,
} from "./ApiButton"
import { mockUIButtonUpdates } from "./ApiButton.testhelpers"
import { accessorsForTesting, tiltfileKeyContext } from "./BrowserStorage"
import { HudErrorContextProvider } from "./HudErrorContext"
import {
  boolFieldForUIButton,
  choiceFieldForUIButton,
  disableButton,
  hiddenFieldForUIButton,
  oneUIButton,
  textFieldForUIButton,
} from "./testdata"
import { UIButton, UIButtonStatus, UIInputSpec } from "./types"

const buttonInputsAccessor = accessorsForTesting(
  `apibutton-TestButton`,
  localStorage
)

type ApiButtonProviderProps = {
  setError?: (error: string) => void
}

function ApiButtonProviders({
  children,
  setError,
}: PropsWithChildren<ApiButtonProviderProps>) {
  return (
    <MemoryRouter>
      <HudErrorContextProvider setError={setError ?? (() => {})}>
        <tiltfileKeyContext.Provider value="test">
          <SnackbarProvider>{children}</SnackbarProvider>
        </tiltfileKeyContext.Provider>
      </HudErrorContextProvider>
    </MemoryRouter>
  )
}

// Following the custom render example from RTL:
// https://testing-library.com/docs/react-testing-library/setup/#custom-render
function customRender(
  component: JSX.Element,
  options?: RenderOptions,
  providerProps?: ApiButtonProviderProps
) {
  return render(component, {
    wrapper: ({ children }) => (
      <ApiButtonProviders {...providerProps} children={children} />
    ),
    ...options,
  })
}

describe("ApiButton", () => {
  beforeEach(() => {
    localStorage.clear()
    mockUIButtonUpdates()
    Date.now = jest.fn(() => 1482363367071)
  })

  afterEach(() => {
    localStorage.clear()
    fetchMock.reset()
  })

  it("renders a simple button", () => {
    const uibutton = oneUIButton({ iconName: "flight_takeoff" })
    customRender(<ApiButton uiButton={uibutton} />)

    const buttonElement = screen.getByLabelText(
      `Trigger ${uibutton.spec!.text!}`
    )
    expect(buttonElement).toBeInTheDocument()
    expect(buttonElement).toHaveTextContent(uibutton.spec!.text!)
    expect(screen.getByText(uibutton.spec!.iconName!)).toBeInTheDocument()
  })

  it("sets a hud error when the api request fails", async () => {
    // To add a mocked error response, reset the current mock
    // for UIButton API call and add back the mock for analytics calls
    // Reset the current mock for UIButton to add fake error response
    fetchMock.reset()
    fetchMock.put(
      (url) => url.startsWith("/proxy/apis/tilt.dev/v1alpha1/uibuttons"),
      { throws: "broken!" }
    )

    let error: string | undefined
    const setError = (e: string) => (error = e)
    const uibutton = oneUIButton({})
    customRender(<ApiButton uiButton={uibutton} />, {}, { setError })

    userEvent.click(screen.getByRole("button"))

    await waitFor(() => {
      expect(screen.getByRole("button")).not.toBeDisabled()
    })

    expect(error).toEqual("Error submitting button click: broken!")
  })

  describe("button with visible inputs", () => {
    let uibutton: UIButton
    let inputSpecs: UIInputSpec[]
    beforeEach(() => {
      inputSpecs = [
        textFieldForUIButton("text_field"),
        boolFieldForUIButton("bool_field", false),
        textFieldForUIButton("text_field_with_default", "default text"),
        hiddenFieldForUIButton("hidden_field", "hidden value 1"),
        choiceFieldForUIButton("choice_field", [
          "choice1",
          "choice2",
          "choice3",
        ]),
      ]
      uibutton = oneUIButton({ inputSpecs })
      customRender(<ApiButton uiButton={uibutton} />).rerender
    })

    it("renders the button with inputs", () => {
      expect(
        screen.getByLabelText(`Trigger ${uibutton.spec!.text!}`)
      ).toBeInTheDocument()
    })

    it("shows the modal with inputs when the button is clicked", () => {
      const button = screen.getByLabelText(
        `Trigger ${uibutton.spec!.text!}`
      )
      userEvent.click(button)

      expect(
        screen.getByText(`Configure ${uibutton.spec!.text!}`)
      ).toBeInTheDocument()
    })

    it("only shows inputs for visible inputs", () => {
      // Open the modal by clicking the button
      const button = screen.getByLabelText(
        `Trigger ${uibutton.spec!.text!}`
      )
      userEvent.click(button)

      inputSpecs.forEach((spec) => {
        if (!spec.hidden) {
          expect(screen.getByLabelText(spec.label!)).toBeInTheDocument()
        }
      })
    })

    it("allows an empty text string when there's a default value", async () => {
      // Open the modal by clicking the button
      const button = screen.getByLabelText(
        `Trigger ${uibutton.spec!.text!}`
      )
      userEvent.click(button)

      // Get the input element with the hardcoded default text
      const inputWithDefault = screen.getByDisplayValue("default text")
      userEvent.clear(inputWithDefault)

      // Use the label text to select and verify the input's value
      expect(screen.getByLabelText("text_field_with_default")).toHaveValue("")
    })

    it("submits the current options when the submit button is clicked", async () => {
      // Open the modal by clicking the button
      const button = screen.getByLabelText(
        `Trigger ${uibutton.spec!.text!}`
      )
      userEvent.click(button)

      // Make a couple changes to the inputs
      userEvent.type(screen.getByLabelText("text_field"), "new_value")
      userEvent.click(screen.getByLabelText("bool_field"))
      userEvent.type(screen.getByLabelText("text_field_with_default"), "!!!!")
      userEvent.click(screen.getByLabelText("choice_field"))
      userEvent.click(screen.getByText("choice1"))
      userEvent.click(screen.getByText("choice3"))

      // Click the confirm button in modal
      userEvent.click(screen.getByText("Confirm & Execute"))

      // Wait for the button to be enabled again,
      // which signals successful trigger button response
      await waitFor(
        () =>
          expect(screen.getByLabelText(`Trigger ${uibutton.spec!.text!}`)).not
            .toBeDisabled
      )

      const calls = fetchMock.calls()
      expect(calls.length).toEqual(1)
      const call = calls[0]
      expect(call[0]).toEqual(
        "/proxy/apis/tilt.dev/v1alpha1/uibuttons/TestButton/status"
      )
      expect(call[1]).toBeTruthy()
      expect(call[1]!.method).toEqual("PUT")
      expect(call[1]!.body).toBeTruthy()
      const actualStatus: UIButtonStatus = JSON.parse(
        call[1]!.body!.toString()
      ).status

      const expectedStatus: UIButtonStatus = {
        lastClickedAt: "2016-12-21T23:36:07.071000+00:00",
        inputs: [
          {
            name: inputSpecs[0].name,
            text: {
              value: "new_value",
            },
          },
          {
            name: inputSpecs[1].name,
            bool: {
              value: true,
            },
          },
          {
            name: inputSpecs[2].name,
            text: {
              value: "default text!!!!",
            },
          },
          {
            name: inputSpecs[3].name,
            hidden: {
              value: inputSpecs[3].hidden!.value,
            },
          },
          {
            name: inputSpecs[4].name,
            choice: {
              value: "choice3",
            },
          },
        ],
      }
      expect(actualStatus).toEqual(expectedStatus)
    })

    it("submits default options when the submit button is clicked", async () => {
      // Open the modal
      userEvent.click(screen.getByLabelText(`Trigger ${uibutton.spec!.text!}`))
      
      // Click confirm in modal
      userEvent.click(screen.getByText("Confirm & Execute"))

      // Wait for the modal to close and API call to complete
      await waitFor(
        () =>
          expect(screen.queryByText("Confirm & Execute")).not.toBeInTheDocument()
      )

      const calls = fetchMock.calls()
      expect(calls.length).toEqual(1)
      const call = calls[0]
      expect(call[0]).toEqual(
        "/proxy/apis/tilt.dev/v1alpha1/uibuttons/TestButton/status"
      )
      expect(call[1]).toBeTruthy()
      expect(call[1]!.method).toEqual("PUT")
      expect(call[1]!.body).toBeTruthy()
      const actualStatus: UIButtonStatus = JSON.parse(
        call[1]!.body!.toString()
      ).status

      const expectedStatus: UIButtonStatus = {
        lastClickedAt: "2016-12-21T23:36:07.071000+00:00",
        inputs: [
          {
            name: inputSpecs[0].name,
            text: {},
          },
          {
            name: inputSpecs[1].name,
            bool: {
              value: false,
            },
          },
          {
            name: inputSpecs[2].name,
            text: {
              value: "default text",
            },
          },
          {
            name: inputSpecs[3].name,
            hidden: {
              value: inputSpecs[3].hidden!.value,
            },
          },
          {
            name: inputSpecs[4].name,
            choice: {
              value: "choice1",
            },
          },
        ],
      }
      expect(actualStatus).toEqual(expectedStatus)
    })
  })

  describe("local storage for input values", () => {
    let uibutton: UIButton
    let inputSpecs: UIInputSpec[]
    beforeEach(() => {
      inputSpecs = [
        textFieldForUIButton("text1"),
        boolFieldForUIButton("bool1"),
      ]
      uibutton = oneUIButton({ inputSpecs })

      // Store previous values for input fields
      buttonInputsAccessor.set({
        text1: "text value",
        bool1: true,
      })

      customRender(<ApiButton uiButton={uibutton} />)
    })

    it("are read from local storage", () => {
      // Open the modal
      userEvent.click(
        screen.getByLabelText(`Trigger ${uibutton.spec!.text!}`)
      )

      expect(screen.getByLabelText("text1")).toHaveValue("text value")
      expect(screen.getByLabelText("bool1")).toBeChecked()
    })

    it("are written to local storage when modal is confirmed", () => {
      // Open the modal
      userEvent.click(
        screen.getByLabelText(`Trigger ${uibutton.spec!.text!}`)
      )

      // Type a new value in the text field
      const textField = screen.getByLabelText("text1")
      userEvent.clear(textField)
      userEvent.type(textField, "new value!")

      // Uncheck the boolean field
      userEvent.click(screen.getByLabelText("bool1"))
      
      // Confirm the modal to persist values
      userEvent.click(screen.getByText("Confirm & Execute"))

      // Expect local storage values are updated after confirmation
      expect(buttonInputsAccessor.get()).toEqual({
        text1: "new value!",
        bool1: false,
      })
    })
  })

  describe("button with only hidden inputs", () => {
    let uibutton: UIButton
    beforeEach(() => {
      const inputSpecs = [1, 2, 3].map((i) =>
        hiddenFieldForUIButton(`hidden${i}`, `value${i}`)
      )
      uibutton = oneUIButton({ inputSpecs })
      customRender(<ApiButton uiButton={oneUIButton({ inputSpecs })} />)
    })

    it("doesn't render an options button", () => {
      expect(
        screen.queryByLabelText(`Open ${uibutton.spec!.text!} options`)
      ).not.toBeInTheDocument()
    })

    it("doesn't render any input elements", () => {
      expect(screen.queryAllByRole("input").length).toBe(0)
    })
  })

  describe("buttons that require confirmation", () => {
    let uibutton: UIButton
    let rerender: RenderResult["rerender"]
    beforeEach(() => {
      uibutton = oneUIButton({ requiresConfirmation: true })
      rerender = customRender(<ApiButton uiButton={uibutton} />).rerender
    })

    it("displays 'confirm' and 'cancel' buttons after a single click", () => {
      const buttonBeforeClick = screen.getByLabelText(
        `Trigger ${uibutton.spec!.text!}`
      )
      expect(buttonBeforeClick).toBeInTheDocument()
      expect(buttonBeforeClick).toHaveTextContent(uibutton.spec!.text!)

      userEvent.click(buttonBeforeClick)

      const confirmButton = screen.getByLabelText(
        `Confirm ${uibutton.spec!.text!}`
      )
      expect(confirmButton).toBeInTheDocument()
      expect(confirmButton).toHaveTextContent("Confirm")

      const cancelButton = screen.getByLabelText(
        `Cancel ${uibutton.spec!.text!}`
      )
      expect(cancelButton).toBeInTheDocument()
    })

    it("clicking the 'confirm' button triggers a button API call", async () => {
      // Click the submit button
      userEvent.click(screen.getByLabelText(`Trigger ${uibutton.spec!.text!}`))

      // Expect that it should not have submitted the click to the backend
      expect(fetchMock.calls().length).toEqual(0)

      // Click the confirm submit button
      userEvent.click(screen.getByLabelText(`Confirm ${uibutton.spec!.text!}`))

      // Wait for the button to be enabled again,
      // which signals successful trigger button response
      await waitFor(
        () =>
          expect(screen.getByLabelText(`Trigger ${uibutton.spec!.text!}`)).not
            .toBeDisabled
      )

      // Expect that the click was submitted and the button text resets
      expect(fetchMock.calls().length).toEqual(1)
      expect(
        screen.getByLabelText(`Trigger ${uibutton.spec!.text!}`)
      ).toHaveTextContent(uibutton.spec!.text!)
    })

    it("clicking the 'cancel' button resets the button", () => {
      // Click the submit button
      userEvent.click(screen.getByLabelText(`Trigger ${uibutton.spec!.text!}`))

      // Expect that it should not have submitted the click to the backend
      expect(fetchMock.calls().length).toEqual(0)

      // Click the cancel submit button
      userEvent.click(screen.getByLabelText(`Cancel ${uibutton.spec!.text!}`))

      // Expect that NO click was submitted and the button text resets
      expect(fetchMock.calls().length).toEqual(0)
      expect(
        screen.getByLabelText(`Trigger ${uibutton.spec!.text!}`)
      ).toHaveTextContent(uibutton.spec!.text!)
    })

    // This test makes sure that the `confirming` state resets if a user
    // clicks a toggle button once, then navigates to another resource
    // with a toggle button (which will have a different button name)
    it("resets the `confirming` state when the button's name changes", () => {
      // Click the button and verify the confirmation state
      userEvent.click(screen.getByLabelText(`Trigger ${uibutton.spec!.text!}`))
      expect(
        screen.getByLabelText(`Confirm ${uibutton.spec!.text!}`)
      ).toBeInTheDocument()
      expect(
        screen.getByLabelText(`Cancel ${uibutton.spec!.text!}`)
      ).toBeInTheDocument()

      // Then update the component's props with a new button
      const anotherUIButton = oneUIButton({
        buttonName: "another-button",
        buttonText: "Click another button!",
        requiresConfirmation: true,
      })
      rerender(<ApiButton uiButton={anotherUIButton} />)

      // Verify that the button's confirmation state is reset
      // and displays the new button text
      const updatedButton = screen.getByLabelText(
        `Trigger ${anotherUIButton.spec!.text!}`
      )
      expect(updatedButton).toBeInTheDocument()
      expect(updatedButton).toHaveTextContent(anotherUIButton.spec!.text!)
    })
  })

  describe("helper functions", () => {
    describe("buttonsByComponent", () => {
      it("returns an empty object if there are no buttons", () => {
        expect(buttonsByComponent(undefined)).toStrictEqual(
          new Map<string, ButtonSet>()
        )
      })

      it("returns a map of resources names to button sets", () => {
        const buttons = [
          oneUIButton({ componentID: "frontend", buttonName: "Lint" }),
          oneUIButton({ componentID: "frontend", buttonName: "Compile" }),
          disableButton("frontend", true),
          oneUIButton({ componentID: "backend", buttonName: "Random scripts" }),
          disableButton("backend", false),
          oneUIButton({ componentID: "data-warehouse", buttonName: "Flush" }),
          oneUIButton({ componentID: "" }),
        ]

        const expectedOutput = new Map<string, ButtonSet>([
          [
            "frontend",
            {
              default: [buttons[0], buttons[1]],
              toggleDisable: buttons[2],
            },
          ],
          ["backend", { default: [buttons[3]], toggleDisable: buttons[4] }],
          ["data-warehouse", { default: [buttons[5]] }],
        ])

        expect(buttonsByComponent(buttons)).toStrictEqual(expectedOutput)
      })
    })
  })
})
