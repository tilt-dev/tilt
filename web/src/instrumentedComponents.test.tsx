import { mount } from "enzyme"
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
    it("performs the underlying onClick and also reports analytics", () => {
      let underlyingOnClickCalled = false
      const onClick = () => {
        underlyingOnClickCalled = true
      }
      const button = mount(
        <InstrumentedButton
          analyticsName="ui.web.foo.bar"
          analyticsTags={{ hello: "goodbye" }}
          onClick={onClick}
        >
          Hello
        </InstrumentedButton>
      )

      button.simulate("click")

      expect(underlyingOnClickCalled).toEqual(true)
      expectIncrs({
        name: "ui.web.foo.bar",
        tags: { action: AnalyticsAction.Click, hello: "goodbye" },
      })
    })

    it("works without an underlying onClick", () => {
      const button = mount(
        <InstrumentedButton analyticsName="ui.web.foo.bar">
          Hello
        </InstrumentedButton>
      )

      button.simulate("click")

      expectIncrs({
        name: "ui.web.foo.bar",
        tags: { action: AnalyticsAction.Click },
      })
    })

    it("works without tags", () => {
      const button = mount(
        <InstrumentedButton analyticsName="ui.web.foo.bar">
          Hello
        </InstrumentedButton>
      )

      button.simulate("click")

      expectIncrs({
        name: "ui.web.foo.bar",
        tags: { action: AnalyticsAction.Click },
      })
    })
  })

  describe("instrumented TextField", () => {
    it("reports analytics, debounced, when edited", () => {
      const root = mount(
        <InstrumentedTextField
          analyticsName={"ui.web.TestTextField"}
          analyticsTags={{ foo: "bar" }}
        />
      )
      const tf = root.find(InstrumentedTextField).find('input[type="text"]')
      // two changes in rapid succession should result in only one analytics event
      tf.simulate("change", { target: { value: "foo" } })
      tf.simulate("change", { target: { value: "foobar" } })
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
      const root = mount(
        <>
          <InstrumentedTextField
            id="resourceNameFilter"
            analyticsName={"ui.web.resourceNameFilter"}
            analyticsTags={{ testing: "true" }}
          />
          <InstrumentedTextField
            id="uibuttonInput"
            analyticsName={"ui.web.uibutton.inputValue"}
            analyticsTags={{ testing: "true" }}
          />
        </>
      )
      const allInputFields = root
        .find(InstrumentedTextField)
        .find('input[type="text"]')
      const inputField1 = allInputFields.at(0)
      const inputField2 = allInputFields.at(1)

      // Trigger an event in the first field
      inputField1.simulate("change", { target: { value: "first!" } })

      // Expect that no analytics calls have been made, since the debounce
      // time for the first field has not been met
      jest.advanceTimersByTime(halfDebounce)
      expectIncrs(...[])

      // Trigger an event in the second field
      inputField2.simulate("change", { target: { value: "second!" } })

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
      const root = mount(
        <InstrumentedCheckbox
          analyticsName={"ui.web.TestCheckbox"}
          analyticsTags={{ foo: "bar" }}
        />
      )
      const tf = root.find(InstrumentedCheckbox).find('input[type="checkbox"]')
      tf.simulate("change", { target: { checked: true } })
      expectIncrs({
        name: "ui.web.TestCheckbox",
        tags: { action: AnalyticsAction.Edit, foo: "bar" },
      })
    })
  })
})
