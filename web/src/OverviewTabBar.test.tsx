import { mount } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import OverviewTabBar, { HomeTab, Tab } from "./OverviewTabBar"

it("infers tab from url", () => {
  const root = mount(
    <MemoryRouter initialEntries={["/r/vigoda/overview"]}>
      <OverviewTabBar
        selectedTab="vigoda"
        tabsForTesting={["vigoda", "snack"]}
      />
    </MemoryRouter>
  )

  let homeTab = root.find(HomeTab)
  let tabs = root.find(Tab)

  expect(homeTab).toHaveLength(1)
  expect(tabs).toHaveLength(2)
  expect(tabs.map((tab) => tab.props().to)).toEqual([
    "/r/vigoda/overview",
    "/r/snack/overview",
  ])
})
