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
import { InstrumentedButton } from "./instrumentedComponents"
import LogStore from "./LogStore"
import PathBuilder from "./PathBuilder"
import { DEFAULT_OPTIONS } from "./ResourceListOptionsContext"
import SidebarItem from "./SidebarItem"
import SidebarResources from "./SidebarResources"
import { SidebarBuildButton } from "./SidebarBuildButton"
import { oneResource, tiltfileResource, twoResourceView } from "./testdata"
import { BuildButtonTooltip, startBuild } from "./trigger"
import BuildButton from "./BuildButton"
import { ResourceView, TriggerMode } from "./types"

type UIResource = Proto.v1alpha1UIResource

let pathBuilder = PathBuilder.forTesting("localhost", "/")

let expectClickable = (button: any, expected: boolean) => {
  const ib = button.find(InstrumentedButton)
  expect(ib.hasClass("is-clickable")).toEqual(expected)
  expect(ib.prop("disabled")).toEqual(!expected)
}
let expectManualStartBuildIcon = (button: any, expected: boolean) => {
  let icon = expected ? "start-build-button-manual.svg" : "start-build-button.svg"
  expect(button.find(InstrumentedButton).getDOMNode().innerHTML).toContain(icon)
}
let expectIsSelected = (button: any, expected: boolean) => {
  expect(button.find(InstrumentedButton).hasClass("is-selected")).toEqual(
    expected
  )
}
let expectIsQueued = (button: any, expected: boolean) => {
  expect(button.find(InstrumentedButton).hasClass("is-queued")).toEqual(
    expected
  )
}
let expectWithTooltip = (button: any, expected: string) => {
  expect(button.find('div[role="tooltip"]').prop("title")).toEqual(expected)
}

let newSidebarItem = (r: UIResource): SidebarItem => {
  return new SidebarItem(r, new LogStore())
}

describe("BuildButton", () => {
  beforeEach(() => {
    mockAnalyticsCalls()
    fetchMock.mock("/api/trigger", JSON.stringify({}))
  })

  afterEach(() => {
    cleanupMockAnalyticsCalls()
  })

  it("POSTs to endpoint when clicked", () => {
    const root = mount(
      <SidebarBuildButton
        isSelected={true}
        triggerMode={TriggerMode.TriggerModeManualWithAutoInit}
        hasBuilt={true}
        isBuilding={false}
        hasPendingChanges={false}
        isQueued={false}
        onStartBuild={() => startBuild("doggos")}
        analyticsTags={{ target: "k8s" }}
      />
    )

    let element = root.find(BuildButton).find(InstrumentedButton)
    expect(element).toHaveLength(1)

    let preventDefaulted = false
    element.simulate("click", {
      preventDefault: () => {
        preventDefaulted = true
      },
    })
    expect(preventDefaulted).toEqual(true)

    expectIncrs({
      name: "ui.web.triggerResource",
      tags: { action: AnalyticsAction.Click, target: "k8s" },
    })

    expect(fetchMock.calls().length).toEqual(2)
    expect(fetchMock.calls()[1][0]).toEqual("/api/trigger")
    expect(fetchMock.calls()[1][1]?.method).toEqual("post")
    expect(fetchMock.calls()[1][1]?.body).toEqual(
      JSON.stringify({
        manifest_names: ["doggos"],
        build_reason: 16 /* BuildReasonFlagTriggerWeb */,
      })
    )
  })

  it("disables button when resource is queued", () => {
    const root = mount(
      <SidebarBuildButton
        isSelected={true}
        triggerMode={TriggerMode.TriggerModeManualWithAutoInit}
        hasBuilt={true}
        isBuilding={false}
        hasPendingChanges={false}
        isQueued={true}
        onStartBuild={() => startBuild("doggos")}
        analyticsTags={{ target: "k8s" }}
      />
    )

    let element = root.find(BuildButton).find(InstrumentedButton)
    expect(element).toHaveLength(1)
    element.simulate("click")

    expect(fetchMock.calls().length).toEqual(0)
  })

  it("shows the button for TriggerModeManual", () => {
    const root = mount(
      <SidebarBuildButton
        isSelected={true}
        triggerMode={TriggerMode.TriggerModeManual}
        hasBuilt={false}
        isBuilding={false}
        hasPendingChanges={false}
        isQueued={false}
        onStartBuild={() => startBuild("doggos")}
        analyticsTags={{ target: "k8s" }}
      />
    )

    let element = root.find(BuildButton).find(InstrumentedButton)
    expectManualStartBuildIcon(element, true)

    expect(element).toHaveLength(1)
    element.simulate("click")

    expect(fetchMock.calls().length).toEqual(2)
  })

  it("shows clickable + bold start build button for manual resource with pending changes", () => {
    let items = twoResourceView().uiResources.map(
      (r: UIResource, i: number) => {
        let res = r.status!
        res.triggerMode = TriggerMode.TriggerModeManualWithAutoInit // both manual
        res.currentBuild = {} // not currently building
        if (i == 0) {
          // only first resource has pending changes -- only this one should have class `isDirty`
          res.hasPendingChanges = true
          res.pendingBuildSince = new Date(Date.now()).toISOString()
          res.queued = false
        } else {
          res.hasPendingChanges = false
          res.pendingBuildSince = "0001-01-01T00:00:00Z"
        }

        return newSidebarItem(r)
      }
    )

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    let buttons = root.find(BuildButton)
    expect(buttons).toHaveLength(2)

    let b0 = buttons.at(0) // Manual resource with pending changes
    let b1 = buttons.at(1) // Manual resource, no pending changes

    expectClickable(b0, true)
    expectManualStartBuildIcon(b0, true)
    expectIsQueued(b0, false)
    expectWithTooltip(b0, BuildButtonTooltip.NeedsManualTrigger)

    expectClickable(b1, true)
    expectManualStartBuildIcon(b1, false)
    expectIsQueued(b1, false)
    expectWithTooltip(b1, BuildButtonTooltip.Default)
  })

  it("shows selected start build button for selected resource", () => {
    let items = twoResourceView().uiResources.map(
      (r: UIResource, i: number) => {
        let res = r.status!
        res.triggerMode = TriggerMode.TriggerModeManualWithAutoInit // both manual
        res.currentBuild = {} // not currently building
        if (i == 0) {
          r.metadata = { name: "selected resource" }
        }

        return newSidebarItem(r)
      }
    )

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected="selected resource"
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    let buttons = root.find(BuildButton)
    expect(buttons).toHaveLength(2)

    expectIsSelected(buttons.at(0), true) // Selected resource
    expectIsSelected(buttons.at(1), false) // Non-selected resource
  })

  // A pending resource may mean that a pod is being rolled out, but is not
  // ready yet. In that case, the start build button will delete the pod (cancelling
  // the rollout) and rebuild.
  it("shows bold start build button when pending", () => {
    let items = twoResourceView().uiResources.map(
      (r: UIResource, i: number) => {
        let res = r.status!
        res.currentBuild = {} // not currently building

        if (i == 0) {
          res.hasPendingChanges = true
          res.pendingBuildSince = new Date(Date.now()).toISOString()
          res.queued = false
        } else {
          res.hasPendingChanges = false
          res.pendingBuildSince = "0001-01-01T00:00:00Z"
        }
        return newSidebarItem(r)
      }
    )

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    let buttons = root.find(BuildButton)
    expect(buttons).toHaveLength(2)
    let b0 = buttons.at(0) // Automatic resource with pending changes
    let b1 = buttons.at(1) // Automatic resource, no pending changes

    expectClickable(b0, true)
    expectManualStartBuildIcon(b0, false)
    expectIsQueued(b0, false)
    expectWithTooltip(b0, BuildButtonTooltip.Default)

    expectClickable(b1, true)
    expectManualStartBuildIcon(b1, false)
    expectIsQueued(b1, false)
    expectWithTooltip(b1, BuildButtonTooltip.Default)
  })

  it("start build button not clickable if resource is building", () => {
    let res = oneResource({ isBuilding: true })
    res.status!.queued = false
    let items = [newSidebarItem(res)]

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    let button = root.find(BuildButton)
    expect(button).toHaveLength(1)

    expectClickable(button, false)
    expectManualStartBuildIcon(button, false)
    expectIsQueued(button, false)
    expectWithTooltip(button, BuildButtonTooltip.UpdateInProgOrPending)
  })

  it("start build button not clickable if resource waiting for first build", () => {
    let r = oneResource({})
    r.status!.buildHistory = []
    r.status!.lastDeployTime = ""
    let items = [newSidebarItem(r)]

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    let button = root.find(BuildButton)
    expect(button).toHaveLength(1)

    expectClickable(button, false)
    expectManualStartBuildIcon(button, false)
    expectIsQueued(button, false)
    expectWithTooltip(button, BuildButtonTooltip.UpdateInProgOrPending)
  })

  it("renders queued resource with class .isQueued and NOT .clickable", () => {
    let res = oneResource({ isBuilding: true })
    let items = [newSidebarItem(res)]

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    let button = root.find(BuildButton)
    expect(button).toHaveLength(1)

    expectClickable(button, false)
    expectManualStartBuildIcon(button, false)
    expectIsQueued(button, true)
    expectWithTooltip(button, BuildButtonTooltip.AlreadyQueued)
  })

  it("shows a start build button for resource that failed its initial build", () => {
    let res = oneResource({ isBuilding: true })
    res.status!.currentBuild = {}
    res.status!.queued = false
    let items = [newSidebarItem(res)]

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    let button = root.find(BuildButton)
    expect(button).toHaveLength(1)

    expectClickable(button, true)
    expectManualStartBuildIcon(button, false)
    expectIsQueued(button, false)
    expectWithTooltip(button, BuildButtonTooltip.Default)
  })

  it("shows start build button for Tiltfile", () => {
    let res = tiltfileResource()
    let items = [newSidebarItem(res)]

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    let button = root.find(BuildButton)
    expect(button).toHaveLength(1)

    expectClickable(button, true)
    expectManualStartBuildIcon(button, false)
    expectIsQueued(button, false)
  })
})
