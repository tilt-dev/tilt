import { Button, Icon, TextField } from "@material-ui/core"
import { mount } from "enzyme"
import fetchMock from "fetch-mock"
import React from "react"
import { act } from "react-dom/test-utils"
import {
  cleanupMockAnalyticsCalls,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import {
  ApiButton,
  ApiButtonForm,
  ApiButtonInputsToggleButton,
  ApiButtonLabel,
} from "./ApiButton"
import { makeUIButton, textField } from "./ApiButton.testhelpers"
import { flushPromises } from "./promise"

type UIButtonStatus = Proto.v1alpha1UIButtonStatus

describe("ApiButton", () => {
  beforeEach(() => {
    fetchMock.reset()
    mockAnalyticsCalls()
    fetchMock.mock(
      (url) => url.startsWith("/proxy/apis/tilt.dev/v1alpha1/uibuttons"),
      JSON.stringify({})
    )
    Date.now = jest.fn(() => 1482363367071)
  })

  afterEach(() => {
    cleanupMockAnalyticsCalls()
  })

  it("renders a simple button", () => {
    const b = makeUIButton()
    const root = mount(<ApiButton button={b} />)
    const button = root.find(ApiButton).find("button")
    expect(button.length).toEqual(1)
    expect(button.find(Icon).text()).toEqual(b.spec!.iconName)
    expect(button.find(ApiButtonLabel).text()).toEqual(b.spec!.text)
  })

  it("renders an options button when the button has inputs", () => {
    const inputs = [1, 2, 3].map((i) => textField(`text${i}`))
    const b = makeUIButton({ inputSpecs: inputs })
    const root = mount(<ApiButton button={b} />)
    expect(
      root.find(ApiButton).find(ApiButtonInputsToggleButton).length
    ).toEqual(1)
  })

  it("shows the options form when the options button is clicked", () => {
    const inputs = [1, 2, 3].map((i) => textField(`text${i}`))
    const b = makeUIButton({ inputSpecs: inputs })
    const root = mount(<ApiButton button={b} />)

    const optionsButton = root.find(ApiButtonInputsToggleButton)
    optionsButton.simulate("click")
    root.update()

    const optionsForm = root.find(ApiButtonForm)
    expect(optionsForm.length).toEqual(1)

    const expectedInputNames = inputs.map((i) => i.label)
    const actualInputNames = optionsForm
      .find(TextField)
      .map((i) => i.prop("label"))
    expect(actualInputNames).toEqual(expectedInputNames)
  })

  it("submits the current options when the submit button is clicked", async () => {
    const inputSpec = textField("text1")
    const b = makeUIButton({ inputSpecs: [inputSpec] })
    const root = mount(<ApiButton button={b} />)

    const optionsButton = root.find(ApiButtonInputsToggleButton)
    optionsButton.simulate("click")
    root.update()

    const tf = root.find(ApiButtonForm).find("input")
    tf.simulate("change", { target: { value: "new_value" } })
    root.update()

    const submit = root.find(ApiButton).find(Button).at(0)
    await act(async () => {
      submit.simulate("click")
      // the button's onclick updates the button so we need to wait for that to resolve
      // within the act() before continuing
      // some related info: https://github.com/testing-library/react-testing-library/issues/281
      await flushPromises()
    })
    root.update()

    const calls = fetchMock
      .calls()
      .filter((c) => c[0] !== "http://localhost/api/analytics")
    expect(calls.length).toEqual(1)
    const call = calls[0]
    expect(call[0]).toEqual(
      "/proxy/apis/tilt.dev/v1alpha1/uibuttons/TestButton/status"
    )
    expect(call[1]).toBeTruthy()
    expect(call[1]!.method).toEqual("PUT")
    expect(call[1]!.body).toBeTruthy()
    const actualStatus: UIButtonStatus = JSON.parse(call[1]!.body!.toString())
      .status

    const expectedStatus: UIButtonStatus = {
      lastClickedAt: "2016-12-21T23:36:07.071000+00:00",
      inputs: [
        {
          name: "text1",
          text: {
            value: "new_value",
          },
        },
      ],
    }
    expect(actualStatus).toEqual(expectedStatus)
  })
})
