import React from "react"
import { MemoryRouter } from "react-router"
import OverviewTabBar from "./OverviewTabBar"
import { OverviewNavProvider } from "./TabNav"

type Resource = Proto.webviewResource

export default {
  title: "OverviewTabBar",
  decorators: [
    (Story: any, context: any) => {
      return (
        <MemoryRouter initialEntries={["/"]}>
          <div style={{ margin: "-1rem" }}>
            <Story />
          </div>
        </MemoryRouter>
      )
    },
  ],
}

export const NoTabs = () => <OverviewTabBar selectedTab={""} />
export const InferOneTab = () => <OverviewTabBar selectedTab={"vigoda"} />

export const TwoTabs = () => {
  let tabs = ["vigoda", "snack"]
  return (
    <OverviewNavProvider tabsForTesting={tabs} validateTab={() => true}>
      <OverviewTabBar selectedTab={""} />
    </OverviewNavProvider>
  )
}

export const TenTabs = () => {
  let tabs = [
    "vigoda_1",
    "vigoda_2",
    "vigoda_3",
    "vigoda_4",
    "vigoda_5",
    "vigoda_6",
    "vigoda_7",
    "vigoda_8",
    "vigoda_9",
    "vigoda_10",
  ]
  return (
    <OverviewNavProvider tabsForTesting={tabs} validateTab={() => true}>
      <OverviewTabBar selectedTab={"vigoda_2"} />
    </OverviewNavProvider>
  )
}

export const LongTabName = () => {
  let tabs = ["extremely-long-tab-name-yes-this-is-very-long"]

  return (
    <OverviewNavProvider tabsForTesting={tabs} validateTab={() => true}>
      <OverviewTabBar selectedTab={""} />
    </OverviewNavProvider>
  )
}
