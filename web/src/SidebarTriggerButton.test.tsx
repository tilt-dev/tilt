import React from "react"
import { mount } from "enzyme"
import SidebarTriggerButton from "./SidebarTriggerButton"
import { TriggerMode } from "./types"

describe("SidebarTriggerButton", () => {
  beforeEach(() => {
    fetchMock.resetMocks()
  })

  it("POSTs to endpoint when clicked", () => {
    fetchMock.mockResponse(JSON.stringify({}))

    const root = mount(
      <SidebarTriggerButton
        isTiltfile={false}
        isSelected={true}
        resourceName="doggos"
        triggerMode={TriggerMode.TriggerModeManualAfterInitial}
        hasBuilt={true}
        isBuilding={false}
        hasPendingChanges={false}
        isQueued={false}
      />
    )

    let element = root.find(".SidebarTriggerButton")
    expect(element).toHaveLength(1)
    element.simulate("click")

    expect(fetchMock.mock.calls.length).toEqual(1)
    expect(fetchMock.mock.calls[0][0]).toEqual("//localhost/api/trigger")
    expect(fetchMock.mock.calls[0][1].method).toEqual("post")
    expect(fetchMock.mock.calls[0][1].body).toEqual(
      JSON.stringify({ manifest_names: ["doggos"] })
    )
  })

  it("disables button when resource is queued", () => {
    fetchMock.mockResponse(JSON.stringify({}))

    const root = mount(
      <SidebarTriggerButton
        isTiltfile={false}
        isSelected={true}
        resourceName="doggos"
        triggerMode={TriggerMode.TriggerModeManualAfterInitial}
        hasBuilt={true}
        isBuilding={false}
        hasPendingChanges={false}
        isQueued={true}
      />
    )

    let element = root.find(".SidebarTriggerButton")
    expect(element).toHaveLength(1)
    element.simulate("click")

    expect(fetchMock.mock.calls.length).toEqual(0)
  })

  it("shows the button for TriggerModeManualIncludingInitial", () => {
    fetchMock.mockResponse(JSON.stringify({}))

    const root = mount(
      <SidebarTriggerButton
        isSelected={true}
        isTiltfile={false}
        resourceName="doggos"
        triggerMode={TriggerMode.TriggerModeManualIncludingInitial}
        hasBuilt={false}
        isBuilding={false}
        hasPendingChanges={false}
        isQueued={false}
      />
    )

    let element = root.find(".SidebarTriggerButton")
    expect(element).toHaveLength(1)
    element.simulate("click")

    expect(fetchMock.mock.calls.length).toEqual(1)
  })
})
