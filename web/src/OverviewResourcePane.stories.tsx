import React from "react"
import { MemoryRouter } from "react-router"
import OverviewResourcePane from "./OverviewResourcePane"
import { nResourceView, tenResourceView, twoResourceView } from "./testdata"

type Resource = Proto.webviewResource

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
  <OverviewResourcePane name={"vigoda"} view={twoResourceView()} />
)

export const TenResources = () => (
  <OverviewResourcePane name={"vigoda_1"} view={tenResourceView()} />
)

export const OneHundredResources = () => (
  <OverviewResourcePane name={"vigoda_1"} view={nResourceView(100)} />
)

export const NotFound = () => (
  <OverviewResourcePane name={"does-not-exist"} view={twoResourceView()} />
)
