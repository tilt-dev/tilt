import React from "react"
import { MemoryRouter } from "react-router"
import OverviewPane from "./OverviewPane"
import PathBuilder from "./PathBuilder"
import { nResourceView, tenResourceView, twoResourceView } from "./testdata"

type Resource = Proto.webviewResource
let pathBuilder = PathBuilder.forTesting("localhost", "/")

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

export const TwoResources = () => (
  <OverviewPane view={twoResourceView()} pathBuilder={pathBuilder} />
)

export const TenResources = () => (
  <OverviewPane view={tenResourceView()} pathBuilder={pathBuilder} />
)

export const OneHundredResources = () => (
  <OverviewPane view={nResourceView(100)} pathBuilder={pathBuilder} />
)
