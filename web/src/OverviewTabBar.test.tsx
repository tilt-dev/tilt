import { mount } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import OverviewTabBar, { Tab } from "./OverviewTabBar"
import PathBuilder from "./PathBuilder"

let pathBuilder = PathBuilder.forTesting("localhost", "/")

it("infers tab from url", () => {
  const root = mount(
    <MemoryRouter initialEntries={["/r/vigoda/overview"]}>
      <OverviewTabBar
        pathBuilder={pathBuilder}
        tabsForTesting={["vigoda", "snack"]}
      />
    </MemoryRouter>
  )

  let tabs = root.find(Tab)
  expect(tabs).toHaveLength(3)
  expect(tabs.map((tab) => tab.props().to)).toEqual([
    "/overview",
    "/r/vigoda/overview",
    "/r/snack/overview",
  ])
})
