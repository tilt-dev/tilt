import React from "react"
import { MemoryRouter } from "react-router"
import OverviewGrid from "./OverviewGrid"
import { nResourceView, tenResourceView, twoResourceView } from "./testdata"

type Resource = Proto.webviewResource

export default {
  title: "OverviewGrid",
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

export const TwoResources = () => <OverviewGrid view={twoResourceView()} />

export const TenResources = () => {
  return <OverviewGrid view={tenResourceView()} />
}

export const OneHundredResources = () => {
  return <OverviewGrid view={nResourceView(100)} />
}
