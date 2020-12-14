import React from "react"
import { MemoryRouter } from "react-router"
import OverviewResourceDetails from "./OverviewResourceDetails"
import PathBuilder from "./PathBuilder"
import { oneResource } from "./testdata"

type Resource = Proto.webviewResource
let pathBuilder = PathBuilder.forTesting("localhost", "/")

export default {
  title: "OverviewResourceDetails",
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

export const Vigoda = () => (
  <OverviewResourceDetails view={oneResource()} pathBuilder={pathBuilder} />
)
