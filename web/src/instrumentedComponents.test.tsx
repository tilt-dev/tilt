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
