import React from "react"
import { MemoryRouter } from "react-router"
import OverviewTable from "./OverviewTable"
import { nResourceView, tenResourceView, twoResourceView } from "./testdata"

export default {
  title: "New UI/Overview/OverviewTable",
  decorators: [
    (Story: any) => (
      <MemoryRouter initialEntries={["/"]}>
        <div style={{ margin: "-1rem" }}>
          <Story />
        </div>
      </MemoryRouter>
    ),
  ],
}

export const TwoResources = () => <OverviewTable view={twoResourceView()} />

export const TenResources = () => {
  return <OverviewTable view={tenResourceView()} />
}

export const OneHundredResources = () => {
  return <OverviewTable view={nResourceView(100)} />
}
