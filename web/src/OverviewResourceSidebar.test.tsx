import { mount } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import { TenResources } from "./OverviewResourceSidebar.stories"

it("renders 10 item resource sidebar", () => {
  mount(<MemoryRouter initialEntries={["/"]}>{TenResources()}</MemoryRouter>)
})
