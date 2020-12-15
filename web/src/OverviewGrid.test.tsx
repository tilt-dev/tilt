import { mount } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import { TenResources } from "./OverviewGrid.stories"

it("renders 10 item grid", () => {
  mount(<MemoryRouter initialEntries={["/"]}>{TenResources()}</MemoryRouter>)
})
