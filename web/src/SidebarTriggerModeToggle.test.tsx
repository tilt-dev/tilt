import { mount } from "enzyme"
import fetchMock from "fetch-mock"
import React from "react"
import { MemoryRouter } from "react-router"
import { AnalyticsAction } from "./analytics"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import LogStore from "./LogStore"
import PathBuilder from "./PathBuilder"
import { DEFAULT_OPTIONS } from "./ResourceListOptionsContext"
import SidebarItem from "./SidebarItem"
import SidebarResources from "./SidebarResources"
import {
  SidebarTriggerModeToggle,
  StyledSidebarTriggerModeToggle,
  ToggleTriggerModeTooltip,
} from "./SidebarTriggerModeToggle"
import { oneTestResource, twoResourceView } from "./testdata"
import { toggleTriggerMode } from "./trigger"
import { ResourceView, TriggerMode } from "./types"

let pathBuilder = PathBuilder.forTesting("localhost", "/")

let expectToggleToAuto = function (mode: TriggerMode) {
  expect(mode).toEqual(TriggerMode.TriggerModeAuto)
}
let expectToggleToManual = function (mode: TriggerMode) {
  expect(mode).toEqual(TriggerMode.TriggerModeManual)
}

describe("SidebarTriggerButton", () => {
  beforeEach(() => {
    fetchMock.reset()
    mockAnalyticsCalls()
  })

  afterEach(() => {
    cleanupMockAnalyticsCalls()
  })

  it("shows toggle button only for test cards", () => {
    let ls = new LogStore()
    let view = twoResourceView()
    view.uiResources.push(oneTestResource())
    let items = view.uiResources.map((r) => new SidebarItem(r, ls))

    const root = mount(
      <MemoryRouter>
        <SidebarResources
          items={items}
          selected={""}
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    // three resources but only one is a test, so we expect only one TriggerModeToggle button
    expect(root.find(SidebarTriggerModeToggle)).toHaveLength(1)
  })

  it("shows different icon depending on current trigger mode", () => {
    let resources = [
      oneTestResource("auto_auto-init"),
      oneTestResource("auto_no-init"),
      oneTestResource("manual_auto-init"),
      oneTestResource("manual_no-init"),
    ]
    resources[0].status!.triggerMode = TriggerMode.TriggerModeAuto
    resources[1].status!.triggerMode = TriggerMode.TriggerModeAutoWithManualInit
    resources[2].status!.triggerMode = TriggerMode.TriggerModeManualWithAutoInit
    resources[3].status!.triggerMode = TriggerMode.TriggerModeManual

    let view = { uiResources: resources }
    let ls = new LogStore()
    let items = view.uiResources.map((r) => new SidebarItem(r, ls))
    const root = mount(
      <MemoryRouter>
        <SidebarResources
          items={items}
          selected={""}
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    let toggles = root.find(StyledSidebarTriggerModeToggle)
    expect(toggles).toHaveLength(4)

    for (let i = 0; i < toggles.length; i++) {
      let button = toggles.at(i)
      let isManual = button.hasClass("is-manual")
      if (i <= 1) {
        // Toggle button for a resource with TriggerModeAuto...
        expect(button.prop("title")).toEqual(ToggleTriggerModeTooltip.isAuto)
        expect(isManual).toBeFalsy()
      } else {
        // Toggle button for a resource with TriggerModeManual...
        expect(button.prop("title")).toEqual(ToggleTriggerModeTooltip.isManual)
        expect(isManual).toBeTruthy()
      }
    }
  })

  it("POSTs to endpoint when clicked", () => {
    fetchMock.mock("/api/override/trigger_mode", JSON.stringify({}))

    let toggleFoobar = toggleTriggerMode.bind(null, "foobar")
    const root = mount(
      <SidebarTriggerModeToggle
        triggerMode={TriggerMode.TriggerModeAuto}
        onModeToggle={toggleFoobar}
      />
    )

    let element = root.find(SidebarTriggerModeToggle)
    expect(element).toHaveLength(1)

    let preventDefaulted = false
    element.simulate("click", {
      preventDefault: () => {
        preventDefaulted = true
      },
    })
    expect(preventDefaulted).toEqual(true)

    expect(fetchMock.calls().length).toEqual(2) // 1 call to analytics, one to /override
    expectIncrs({
      name: "ui.web.toggleTriggerMode",
      tags: {
        action: AnalyticsAction.Click,
        toMode: TriggerMode.TriggerModeManual.toString(),
      },
    })

    expect(fetchMock.calls()[1][0]).toEqual("/api/override/trigger_mode")
    expect(fetchMock.calls()[1][1]?.method).toEqual("post")
    expect(fetchMock.calls()[1][1]?.body).toEqual(
      JSON.stringify({
        manifest_names: ["foobar"],
        trigger_mode: TriggerMode.TriggerModeManual,
      })
    )
  })

  it("toggles auto to manual", () => {
    const root = mount(
      <SidebarTriggerModeToggle
        triggerMode={TriggerMode.TriggerModeAuto}
        onModeToggle={expectToggleToManual}
      />
    )

    let element = root.find(SidebarTriggerModeToggle)
    expect(element).toHaveLength(1)

    element.simulate("click")
  })

  it("toggles manualAfterInitial to auto", () => {
    const root = mount(
      <SidebarTriggerModeToggle
        triggerMode={TriggerMode.TriggerModeManualWithAutoInit}
        onModeToggle={expectToggleToAuto}
      />
    )

    let element = root.find(SidebarTriggerModeToggle)
    expect(element).toHaveLength(1)

    element.simulate("click")
  })

  it("toggles manualIncludingInitial to auto", () => {
    const root = mount(
      <SidebarTriggerModeToggle
        triggerMode={TriggerMode.TriggerModeManual}
        onModeToggle={expectToggleToAuto}
      />
    )

    let element = root.find(SidebarTriggerModeToggle)
    expect(element).toHaveLength(1)

    element.simulate("click")
  })
})
