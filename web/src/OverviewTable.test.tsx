import { mount } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import { CustomActionButton } from "./OverviewButton"
import OverviewTable, { ResourceTableRow } from "./OverviewTable"
import { TenResources } from "./OverviewTable.stories"
import { nResourceView, oneButton } from "./testdata"

it("renders 10 item in table", () => {
  mount(<MemoryRouter initialEntries={["/"]}>{TenResources()}</MemoryRouter>)
})

it("shows buttons on the appropriate resources", () => {
  let view = nResourceView(3)
  // one resource with one button, one with multiple, and one with none
  view.uiButtons = [
    oneButton(0, view.uiResources[0].metadata?.name!),
    oneButton(1, view.uiResources[1].metadata?.name!),
    oneButton(2, view.uiResources[1].metadata?.name!),
  ]

  const root = mount(
    <MemoryRouter initialEntries={["/"]}>
      <OverviewTable view={view} />
    </MemoryRouter>
  )

  // buttons expected to be on each row, in order
  const expectedButtons = [["button1"], ["button2", "button3"], []]
  // first row is headers, so skip it
  const rows = root.find(ResourceTableRow).slice(1)
  const actualButtons = rows.map((row) =>
    row.find(CustomActionButton).map((e) => e.prop("button").metadata.name)
  )

  expect(actualButtons).toEqual(expectedButtons)
})
