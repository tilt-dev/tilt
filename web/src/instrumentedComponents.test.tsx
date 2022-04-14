import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React from "react"
import { AnalyticsAction } from "./analytics"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import {
  InstrumentedButton,
  InstrumentedCheckbox,
  InstrumentedTextField,
  textFieldEditDebounceMilliseconds,
} from "./instrumentedComponents"

describe("instrumented components", () => {
  beforeEach(() => {
    mockAnalyticsCalls()
    jest.useFakeTimers()
  })

  afterEach(() => {
    cleanupMockAnalyticsCalls()
    jest.useRealTimers()
  })

  describe("instrumented button", () => {
    it("reports analytics with default tags and correct name", () => {
      render(
        <InstrumentedButton analyticsName="ui.web.foo.bar">
          Hello
        </InstrumentedButton>
      )

      userEvent.click(screen.getByRole("button"))

      expectIncrs({
        name: "ui.web.foo.bar",
        tags: { action: AnalyticsAction.Click },
      })
    })

    it("reports analytics with any additional custom tags", () => {
      const customTags = { hello: "goodbye" }
      render(
        <InstrumentedButton
          analyticsName="ui.web.foo.bar"
          analyticsTags={customTags}
        >
          Hello
        </InstrumentedButton>
      )

      userEvent.click(screen.getByRole("button"))

      expectIncrs({
        name: "ui.web.foo.bar",
        tags: { action: AnalyticsAction.Click, ...customTags },
      })
    })

    it("invokes the click callback when provided", () => {
      const onClickSpy = jest.fn()
      render(
        <InstrumentedButton
          analyticsName="ui.web.foo.bar"
          analyticsTags={{ hello: "goodbye" }}
          onClick={onClickSpy}
        >
          Hello
        </InstrumentedButton>
      )

      expect(onClickSpy).not.toBeCalled()

      userEvent.click(screen.getByRole("button"))

      expect(onClickSpy).toBeCalledTimes(1)
    })
  })

  describe("instrumented TextField", () => {
    it("reports analytics, debounced, when edited", () => {
      render(
        <InstrumentedTextField
          analyticsName={"ui.web.TestTextField"}
          analyticsTags={{ foo: "bar" }}
          InputProps={{ "aria-label": "Help search" }}
        />
      )

      const inputField = screen.getByLabelText("Help search")
      // two changes in rapid succession should result in only one analytics event
      userEvent.type(inputField, "foo")
      userEvent.type(inputField, "bar")

      expectIncrs(...[])
      jest.advanceTimersByTime(10000)
      expectIncrs({
        name: "ui.web.TestTextField",
        tags: { action: AnalyticsAction.Edit, foo: "bar" },
      })
    })

    // This test is to make sure that the debounce interval is not
    // shared between instances of the same debounced component.
    // When a user edits one text field and then edits another,
    // the debounce internal should start and operate independently
    // for each text field.
    it("debounces analytics for text fields on an instance-by-instance basis", () => {
      const halfDebounce = textFieldEditDebounceMilliseconds / 2
      render(
        <>
          <InstrumentedTextField
            id="resourceNameFilter"
            analyticsName={"ui.web.resourceNameFilter"}
            analyticsTags={{ testing: "true" }}
            InputProps={{ "aria-label": "Resource name filter" }}
          />
          <InstrumentedTextField
            id="uibuttonInput"
            analyticsName={"ui.web.uibutton.inputValue"}
            analyticsTags={{ testing: "true" }}
            InputProps={{ "aria-label": "Button value" }}
          />
        </>
      )

      // Trigger an event in the first field
      userEvent.type(screen.getByLabelText("Resource name filter"), "first!")

      // Expect that no analytics calls have been made, since the debounce
      // time for the first field has not been met
      jest.advanceTimersByTime(halfDebounce)
      expectIncrs(...[])

      // Trigger an event in the second field
      userEvent.type(screen.getByLabelText("Button value"), "second!")

      // Expect that _only_ the first field's analytics event has occurred,
      // since that debounce interval has been met for the first field.
      // If the debounce was shared between multiple instances of the text
      // field, this analytics call wouldn't occur.
      jest.advanceTimersByTime(halfDebounce)
      expectIncrs({
        name: "ui.web.resourceNameFilter",
        tags: { action: AnalyticsAction.Edit, testing: "true" },
      })

      // Expect that the second field's analytics event has occurred, now
      // that the debounce interval has been met for the first field
      jest.advanceTimersByTime(halfDebounce)
      expectIncrs(
        {
          name: "ui.web.resourceNameFilter",
          tags: { action: AnalyticsAction.Edit, testing: "true" },
        },
        {
          name: "ui.web.uibutton.inputValue",
          tags: { action: AnalyticsAction.Edit, testing: "true" },
        }
      )
    })
  })

  describe("instrumented Checkbox", () => {
    it("reports analytics when clicked", () => {
      render(
        <InstrumentedCheckbox
          analyticsName={"ui.web.TestCheckbox"}
          analyticsTags={{ foo: "bar" }}
          inputProps={{ "aria-label": "Check me" }}
        />
      )

      userEvent.click(screen.getByLabelText("Check me"))
      expectIncrs({
        name: "ui.web.TestCheckbox",
        tags: { action: AnalyticsAction.Edit, foo: "bar" },
      })
    })
  })
})
