import { mount } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router-dom"
import { DialogLegacy, DialogOverview } from "./HelpDialog.stories"

it("renders overview dialog", () => {
  mount(
    <MemoryRouter initialEntries={["/"]}>
      <DialogOverview />
    </MemoryRouter>
  )
})

it("renders legacy dialog", () => {
  mount(
    <MemoryRouter initialEntries={["/"]}>
      <DialogLegacy />
    </MemoryRouter>
  )
})
