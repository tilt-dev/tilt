import { mount } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import OverviewTabBar, { HomeTab, Tab } from "./OverviewTabBar"
import { OverviewNavProvider, TabNavContextConsumer } from "./TabNav"
import { ResourceView } from "./types"

it("propagate selected tab", () => {
  let capturedNav: any = null
  const root = mount(
    <MemoryRouter initialEntries={["/r/vigoda/overview"]}>
      <OverviewNavProvider
        resourceView={ResourceView.Grid}
        tabsForTesting={["vigoda", "snack"]}
      >
        <OverviewTabBar selectedTab="vigoda" />
        <TabNavContextConsumer>
          {(nav) => void (capturedNav = nav)}
        </TabNavContextConsumer>
      </OverviewNavProvider>
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

  expect(capturedNav?.selectedTab).toEqual("vigoda")
})
