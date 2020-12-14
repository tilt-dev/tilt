import React from "react"
import { MemoryRouter } from "react-router"
import OverviewResourceSidebar from "./OverviewResourceSidebar"
import PathBuilder from "./PathBuilder"
import { twoResourceView } from "./testdata"

type Resource = Proto.webviewResource
let pathBuilder = PathBuilder.forTesting("localhost", "/")

export default {
  title: "OverviewResourceSidebar",
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
  <OverviewResourceSidebar
    name={"vigoda"}
    view={twoResourceView()}
    pathBuilder={pathBuilder}
  />
)
