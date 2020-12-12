import React from "react"
import { MemoryRouter } from "react-router"
import OverviewResourceBar from "./OverviewResourceBar"
import PathBuilder from "./PathBuilder"
import { twoResourceView } from "./testdata"

type Resource = Proto.webviewResource
let pathBuilder = PathBuilder.forTesting("localhost", "/")

export default {
  title: "OverviewResourceBar",
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
  <OverviewResourceBar view={twoResourceView()} pathBuilder={pathBuilder} />
)
