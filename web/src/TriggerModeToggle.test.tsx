import { mount } from "enzyme"
import fetchMock from "jest-fetch-mock"
import React from "react"
import { MemoryRouter } from "react-router"
import { expectIncr } from "./analytics_test_helpers"
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
  expect(mode).toEqual(TriggerMode.TriggerModeManualAfterInitial)
}

describe("SidebarTriggerButton", () => {
  beforeEach(() => {
    fetchMock.resetMocks()
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
      oneResourceTestWithName("auto"),
      oneResourceTestWithName("manual-after-initial"),
      oneResourceTestWithName("manual-incl-initial"),
    ]
    resources[0].triggerMode = TriggerMode.TriggerModeAuto
    resources[1].triggerMode = TriggerMode.TriggerModeManualAfterInitial
    resources[2].triggerMode = TriggerMode.TriggerModeManualIncludingInitial

    let view = { resources: resources }

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        {<OverviewPane view={view} />}
      </MemoryRouter>
    )

    let toggles = root.find(TriggerModeToggleRoot)
    expect(toggles).toHaveLength(3)

    for (let i = 0; i < toggles.length; i++) {
      let button = toggles.at(i)
      let themeProvider = button.parent()
      let isManual = themeProvider.props().theme.isManualTriggerMode
      if (i == 0) {
        // Toggle button for a resource with TriggerModeAuto
        expect(button.prop("title")).toEqual(ToggleTriggerModeTooltip.isManual)
        expect(isManual).toBeFalsy()
      } else {
        // Toggle button for a resource with TriggerModeManual...
        expect(button.prop("title")).toEqual(ToggleTriggerModeTooltip.isAuto)
        expect(isManual).toBeTruthy()
      }
    }
  })

  it("POSTs to endpoint when clicked", () => {
    fetchMock.mockResponse(JSON.stringify({}))

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

    expect(fetchMock.mock.calls.length).toEqual(2) // 1 call to analytics, one to /override
    expectIncr(0, "ui.web.toggleTriggerMode", { toMode: "1" })

    expect(fetchMock.mock.calls[1][0]).toEqual("/api/override/trigger_mode")
    expect(fetchMock.mock.calls[1][1]?.method).toEqual("post")
    expect(fetchMock.mock.calls[1][1]?.body).toEqual(
      JSON.stringify({
        manifestNames: ["foobar"],
        triggerMode: 1,
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
        triggerMode={TriggerMode.TriggerModeManualAfterInitial}
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
        triggerMode={TriggerMode.TriggerModeManualIncludingInitial}
        onModeToggle={expectToggleToAuto}
      />
    )

    let element = root.find(TriggerModeToggle)
    expect(element).toHaveLength(1)

    element.simulate("click")
  })
})
