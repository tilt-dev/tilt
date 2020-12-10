import React from "react"
import { MemoryRouter } from "react-router"
import OverviewTabBar from "./OverviewTabBar"
import PathBuilder from "./PathBuilder"
import { twoResourceView } from "./testdata"

type Resource = Proto.webviewResource
let pathBuilder = PathBuilder.forTesting("localhost", "/")

export default {
  title: "OverviewTabBar",
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
  <OverviewTabBar view={twoResourceView()} pathBuilder={pathBuilder} />
)
