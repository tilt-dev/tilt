import { mount } from "enzyme"
import { MemoryRouter } from "react-router-dom"
import { NotFound } from "./OverviewResourcePane.stories"
import OverviewResourceSidebar from "./OverviewResourceSidebar"

it("renders correctly when no resource found", () => {
  let root = mount(
    <MemoryRouter initialEntries={["/"]}>{NotFound()}</MemoryRouter>
  )
  let el = root.getDOMNode()
  expect(el.innerHTML).toEqual(
    expect.stringContaining("No resource 'does-not-exist'")
  )

  expect(root.find(OverviewResourceSidebar)).toHaveLength(1)
})
