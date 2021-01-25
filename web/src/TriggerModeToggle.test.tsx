import { mount } from "enzyme"
import fetchMock from "jest-fetch-mock"
import React from "react"
import { MemoryRouter } from "react-router"
import OverviewPane from "./OverviewPane"
import PathBuilder from "./PathBuilder"
import {
  oneResourceTest,
  oneResourceTestWithName,
  twoResourceView,
} from "./testdata"
import { TriggerModeToggle, TriggerModeToggleStyle } from "./TriggerModeToggle"
import { TriggerMode } from "./types"

type Resource = Proto.webviewResource

let pathBuilder = PathBuilder.forTesting("localhost", "/")

let expectClickable = (button: any, expected: boolean) => {
  expect(button.hasClass("clickable")).toEqual(expected)
  expect(button.prop("disabled")).toEqual(!expected)
}
let expectClickMe = (button: any, expected: boolean) => {
  expect(button.hasClass("clickMe")).toEqual(expected)
}
let expectIsSelected = (button: any, expected: boolean) => {
  expect(button.hasClass("isSelected")).toEqual(expected)
}
let expectIsQueued = (button: any, expected: boolean) => {
  expect(button.hasClass("isQueued")).toEqual(expected)
}
let expectWithTooltip = (button: any, expected: string) => {
  expect(button.prop("title")).toEqual(expected)
}

describe("SidebarTriggerButton", () => {
  beforeEach(() => {
    fetchMock.resetMocks()
  })

  // it("POSTs to endpoint when clicked", () => {
  //   fetchMock.mockResponse(JSON.stringify({}))
  //
  //   const root = mount(
  //     <SidebarTriggerButton
  //       isTiltfile={false}
  //       isSelected={true}
  //       triggerMode={TriggerMode.TriggerModeManualAfterInitial}
  //       hasBuilt={true}
  //       isBuilding={false}
  //       hasPendingChanges={false}
  //       isQueued={false}
  //       onTrigger={() => triggerUpdate("doggos", "click")}
  //     />
  //   )
  //
  //   let element = root.find("button.SidebarTriggerButton")
  //   expect(element).toHaveLength(1)
  //
  //   let preventDefaulted = false
  //   element.simulate("click", {
  //     preventDefault: () => {
  //       preventDefaulted = true
  //     },
  //   })
  //   expect(preventDefaulted).toEqual(true)
  //
  //   expect(fetchMock.mock.calls.length).toEqual(2)
  //   expectIncr(0, "ui.web.triggerResource", { action: "click" })
  //
  //   expect(fetchMock.mock.calls[1][0]).toEqual("//localhost/api/trigger")
  //   expect(fetchMock.mock.calls[1][1]?.method).toEqual("post")
  //   expect(fetchMock.mock.calls[1][1]?.body).toEqual(
  //     JSON.stringify({
  //       manifest_names: ["doggos"],
  //       build_reason: 16 /* BuildReasonFlagTriggerWeb */,
  //     })
  //   )
  // }

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

    let toggles = root.find(TriggerModeToggleStyle)
    expect(toggles).toHaveLength(3)

    for (let i = 0; i < toggles.length; i++) {
      let themeProvider = toggles.at(i).parent()
      let isManual = themeProvider.props().theme.isManualTriggerMode
      if (i == 0) {
        expect(isManual).toBeFalsy()
      } else {
        expect(isManual).toBeTruthy()
      }
    }
  })
})
