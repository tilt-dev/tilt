import { mount } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import { TenResources } from "./OverviewTable.stories"

it("renders 10 item in table", () => {
  mount(<MemoryRouter initialEntries={["/"]}>{TenResources()}</MemoryRouter>)
})
