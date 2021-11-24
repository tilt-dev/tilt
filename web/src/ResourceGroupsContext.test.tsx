import { mount, ReactWrapper } from "enzyme"
import React from "react"
import { AnalyticsAction, AnalyticsType } from "./analytics"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import {
  DEFAULT_GROUP_STATE,
  ResourceGroupsContextProvider,
  useResourceGroups,
} from "./ResourceGroupsContext"

const GROUP_STATE_ID = "test-group-state"
const LABEL_STATE_ID = "test-label-state"

// This is a very basic test component that prints out the state
// from the ResourceGroups context and provides buttons to trigger
// methods returned by the context, so they can be tested
const TestConsumer = (props: { labelName?: string }) => {
  const { groups, getGroup, toggleGroupExpanded } = useResourceGroups()

  return (
    <>
      <p id={GROUP_STATE_ID}>{JSON.stringify(groups)}</p>
      {/* Display the label state if a specific label is present */}
      {props.labelName && (
        <p id={LABEL_STATE_ID}>{JSON.stringify(getGroup(props.labelName))}</p>
      )}
      {/* Display a button to toggle the label state if a specific label is present */}
      {props.labelName && (
        <button
          onClick={() =>
            toggleGroupExpanded(props.labelName || "", AnalyticsType.Grid)
          }
        />
      )}
    </>
  )
}

describe("ResourceGroupsContext", () => {
  let wrapper: ReactWrapper<typeof TestConsumer>

  // Helpers
  const groupState = () => wrapper.find(`#${GROUP_STATE_ID}`).text()
  const labelState = () => wrapper.find(`#${LABEL_STATE_ID}`).text()
  const clickButton = () => {
    wrapper.find("button").simulate("click")
    wrapper.update()
  }

  beforeEach(() => {
    mockAnalyticsCalls()
  })

  afterEach(() => {
    localStorage.clear()
    cleanupMockAnalyticsCalls()
  })

  it("defaults to an empty state with no groups", () => {
    wrapper = mount(
      <ResourceGroupsContextProvider>
        <TestConsumer />
      </ResourceGroupsContextProvider>
    )

    expect(groupState()).toBe(JSON.stringify({}))
  })

  describe("toggleGroupExpanded", () => {
    it("sets expanded to `true` when group is collapsed", () => {
      const testValues = { test: { expanded: false } }
      wrapper = mount(
        <ResourceGroupsContextProvider initialValuesForTesting={testValues}>
          <TestConsumer labelName="test" />
        </ResourceGroupsContextProvider>
      )
      clickButton()

      expect(labelState()).toBe(JSON.stringify({ expanded: true }))
    })

    it("sets expanded to `false` when group is expanded", () => {
      const testValues = { test: { expanded: true } }
      wrapper = mount(
        <ResourceGroupsContextProvider initialValuesForTesting={testValues}>
          <TestConsumer labelName="test" />
        </ResourceGroupsContextProvider>
      )
      clickButton()

      expect(labelState()).toBe(JSON.stringify({ expanded: false }))
    })

    it("sets expanded to `false` if a group isn't saved yet and is toggled", () => {
      wrapper = mount(
        <ResourceGroupsContextProvider>
          <TestConsumer labelName="a-non-existent-group" />
        </ResourceGroupsContextProvider>
      )
      clickButton()

      expect(labelState()).toBe(JSON.stringify({ expanded: false }))
    })

    it("makes an analytics call with the right payload", () => {
      const testValues = { test: { expanded: true } }
      wrapper = mount(
        <ResourceGroupsContextProvider initialValuesForTesting={testValues}>
          <TestConsumer labelName="test" />
        </ResourceGroupsContextProvider>
      )
      clickButton()
      // Expect the "collapse" action value because the test label group is expanded
      // when it's clicked on and the "grid" type value because it's hardcoded in the
      // test component
      expectIncrs({
        name: "ui.web.resourceGroup",
        tags: { action: AnalyticsAction.Collapse, type: AnalyticsType.Grid },
      })
    })
  })

  describe("getGroup", () => {
    it("returns the correct state of a resource group", () => {
      const testValues = { frontend: { expanded: false } }
      wrapper = mount(
        <ResourceGroupsContextProvider initialValuesForTesting={testValues}>
          <TestConsumer labelName="frontend" />
        </ResourceGroupsContextProvider>
      )

      expect(labelState()).toBe(JSON.stringify({ expanded: false }))
    })

    it("returns a default state of a resource group if a group isn't saved yet", () => {
      const testValues = { frontend: { expanded: false } }
      wrapper = mount(
        <ResourceGroupsContextProvider initialValuesForTesting={testValues}>
          <TestConsumer labelName="backend" />
        </ResourceGroupsContextProvider>
      )

      expect(labelState()).toBe(JSON.stringify(DEFAULT_GROUP_STATE))
    })
  })
})
