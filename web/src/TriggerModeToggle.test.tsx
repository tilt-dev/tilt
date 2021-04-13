import { mount } from "enzyme"
import fetchMock from "fetch-mock"
import React from "react"
import { MemoryRouter } from "react-router"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import { toggleTriggerMode } from "./OverviewItemView"
import OverviewPane from "./OverviewPane"
import {
  oneResourceTest,
  oneResourceTestWithName,
  twoResourceView,
} from "./testdata"
import {
  ToggleTriggerModeTooltip,
  TriggerModeToggle,
  TriggerModeToggleRoot,
} from "./TriggerModeToggle"
import { TriggerMode } from "./types"

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
    let view = twoResourceView()
    view.resources.push(oneResourceTest())

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        {<OverviewPane view={view} />}
      </MemoryRouter>
    )

    // three resources but only one is a test, so we expect only one TriggerModeToggle button
    expect(root.find(TriggerModeToggle)).toHaveLength(1)
  })

  it("shows different icon depending on current trigger mode", () => {
    let resources = [
      oneResourceTestWithName("auto_auto-init"),
      oneResourceTestWithName("auto_no-init"),
      oneResourceTestWithName("manual_auto-init"),
      oneResourceTestWithName("manual_no-init"),
    ]
    resources[0].triggerMode = TriggerMode.TriggerModeAuto
    resources[1].triggerMode = TriggerMode.TriggerModeAutoWithManualInit
    resources[2].triggerMode = TriggerMode.TriggerModeManualWithAutoInit
    resources[3].triggerMode = TriggerMode.TriggerModeManual

    let view = { resources: resources }

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        {<OverviewPane view={view} />}
      </MemoryRouter>
    )

    let toggles = root.find(TriggerModeToggleRoot)
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
      <TriggerModeToggle
        triggerMode={TriggerMode.TriggerModeAuto}
        onModeToggle={toggleFoobar}
      />
    )

    let element = root.find(TriggerModeToggle)
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
        action: "click",
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
      <TriggerModeToggle
        triggerMode={TriggerMode.TriggerModeAuto}
        onModeToggle={expectToggleToManual}
      />
    )

    let element = root.find(TriggerModeToggle)
    expect(element).toHaveLength(1)

    element.simulate("click")
  })

  it("toggles manualAfterInitial to auto", () => {
    const root = mount(
      <TriggerModeToggle
        triggerMode={TriggerMode.TriggerModeManualWithAutoInit}
        onModeToggle={expectToggleToAuto}
      />
    )

    let element = root.find(TriggerModeToggle)
    expect(element).toHaveLength(1)

    element.simulate("click")
  })

  it("toggles manualIncludingInitial to auto", () => {
    const root = mount(
      <TriggerModeToggle
        triggerMode={TriggerMode.TriggerModeManual}
        onModeToggle={expectToggleToAuto}
      />
    )

    let element = root.find(TriggerModeToggle)
    expect(element).toHaveLength(1)

    element.simulate("click")
  })
})
