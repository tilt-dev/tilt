import React from "react"
import ReactDOM from "react-dom"
import renderer from "react-test-renderer"
import { MemoryRouter } from "react-router"
import SideBar from "./Sidebar"
import { ResourceView } from "./HUD"

it("renders empty resource list without crashing", () => {
  const tree = renderer
    .create(
      <MemoryRouter initialEntries={["/"]}>
        <SideBar
          isClosed={true}
          items={[]}
          selected=""
          toggleSidebar={null}
          resourceView={ResourceView.Log}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})
