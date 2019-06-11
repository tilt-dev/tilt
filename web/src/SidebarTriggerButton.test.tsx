import React from "react"
import { mount } from "enzyme"
import SidebarTriggerButton from "./SidebarTriggerButton"
import { TriggerMode } from "./types"

it("renders trigger button in manual mode", () => {
  const root = mount(
    <SidebarTriggerButton
      isSelected={true}
      resourceName="doggos"
      triggerMode={TriggerMode.TriggerModeManual}
    />
  )

  expect(root.find(".SidebarTriggerButton")).toHaveLength(1)
})
