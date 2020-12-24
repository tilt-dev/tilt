import React from "react"
import { MemoryRouter } from "react-router"
import OverviewPane from "./OverviewPane"
import { nResourceView, tenResourceView, twoResourceView } from "./testdata"

type Resource = Proto.webviewResource

export default {
  title: "OverviewPane",
  decorators: [
    (Story: any) => (
      <MemoryRouter initialEntries={["/"]}>
        <div style={{ margin: "-1rem", height: "80vh" }}>
          <Story />
        </div>
      </MemoryRouter>
    ),
  ],
}

export const TwoResources = () => <OverviewPane view={twoResourceView()} />

export const TenResources = () => <OverviewPane view={tenResourceView()} />

export const OneHundredResources = () => (
  <OverviewPane view={nResourceView(100)} />
)
