import { mount } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import { CompleteDetails, OneItemHealthy } from "./OverviewItemView.stories"

it("renders overview item", () => {
  mount(<MemoryRouter initialEntries={["/"]}>{OneItemHealthy()}</MemoryRouter>)
})

it("renders complete details popover", () => {
  mount(<MemoryRouter initialEntries={["/"]}>{CompleteDetails()}</MemoryRouter>)
})
