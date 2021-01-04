import React from "react"
import { MemoryRouter } from "react-router"
import OverviewResourceDetails from "./OverviewResourceDetails"
import { oneResource } from "./testdata"

type Resource = Proto.webviewResource

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
  <OverviewResourceDetails view={oneResource()} name="vigoda" />
)
