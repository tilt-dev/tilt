import React from "react"
import { MemoryRouter } from "react-router"
import OverviewTable from "./OverviewTable"
import {
  nButtonView,
  nResourceView,
  tenResourceView,
  twoResourceView,
} from "./testdata"

export default {
  title: "New UI/Overview/OverviewTable",
  decorators: [
    (Story: any) => (
      <MemoryRouter initialEntries={["/"]}>
        {/* required for MUI <Icon> */}
        <link rel="stylesheet" href="https://fonts.googleapis.com/icon?family=Material+Icons" />
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

export const OneButton = () => {
  return <OverviewTable view={nButtonView(1)} />
}

export const TenButtons = () => {
  return <OverviewTable view={nButtonView(10)} />
}
