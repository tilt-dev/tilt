import { mount } from "enzyme"
import { AnalyticsAction } from "./analytics"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import { InstrumentedButton } from "./instrumentedComponents"

describe("instrumented components", () => {
  beforeEach(() => {
    mockAnalyticsCalls()
  })

  afterEach(() => {
    cleanupMockAnalyticsCalls()
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
})
