import React from "react"
import { MemoryRouter } from "react-router"
import OverviewGrid from "./OverviewGrid"
import PathBuilder from "./PathBuilder"
import { tenResourceView, twoResourceView } from "./testdata"

type Resource = Proto.webviewResource
let pathBuilder = PathBuilder.forTesting("localhost", "/")

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

export const TwoResources = () => (
  <OverviewGrid view={twoResourceView()} pathBuilder={pathBuilder} />
)

export const TenResources = () => {
  return <OverviewGrid view={tenResourceView()} pathBuilder={pathBuilder} />
}
