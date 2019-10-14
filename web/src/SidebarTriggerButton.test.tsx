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
        isSelected={true}
        resourceName="doggos"
        triggerMode={TriggerMode.TriggerModeManual}
        isReady={true}
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
})
