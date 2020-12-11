import React from "react"
import { MemoryRouter } from "react-router"
import OverviewResourcePane from "./OverviewResourcePane"
import PathBuilder from "./PathBuilder"
import { twoResourceView } from "./testdata"

type Resource = Proto.webviewResource
let pathBuilder = PathBuilder.forTesting("localhost", "/")

export default {
  title: "OverviewResourcePane",
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
  <OverviewResourcePane
    name={"vigoda"}
    view={twoResourceView()}
    pathBuilder={pathBuilder}
  />
)
export const NotFound = () => (
  <OverviewResourcePane
    name={"does-not-exist"}
    view={twoResourceView()}
    pathBuilder={pathBuilder}
  />
)
