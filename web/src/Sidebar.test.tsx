import React from "react"
import ReactDOM from "react-dom"
import renderer from "react-test-renderer"
import { MemoryRouter } from "react-router"
import SideBar from "./Sidebar"

it("renders empty resource list without crashing", () => {
  const tree = renderer
    .create(
      <MemoryRouter initialEntries={["/"]}>
        <SideBar isClosed={true} items={[]} selected="" toggleSidebar={null} />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})
