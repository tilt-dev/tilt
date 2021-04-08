import { mount } from "enzyme"
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
        tags: { action: "click", hello: "goodbye" },
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
        tags: { action: "click" },
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
        tags: { action: "click" },
      })
    })
  })
})
