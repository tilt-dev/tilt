import React from "react"
import { MemoryRouter } from "react-router"
import OverviewPane from "./OverviewPane"
import { StarredResourceMemoryProvider } from "./StarredResourcesContext"
import { nResourceView, tenResourceView, twoResourceView } from "./testdata"

type Resource = Proto.webviewResource

export default {
  title: "New UI/OverviewPane",
  decorators: [
    (Story: any) => (
      <MemoryRouter initialEntries={["/"]}>
        <div style={{ margin: "-1rem", height: "80vh" }}>
          <StarredResourceMemoryProvider>
            <Story />
          </StarredResourceMemoryProvider>
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

export const OneHundredResourcesOneStar = () => (
  <StarredResourceMemoryProvider initialValueForTesting={["vigoda_2"]}>
    <OverviewPane view={nResourceView(100)} />
  </StarredResourceMemoryProvider>
)

export const OneHundredResourcesTenStars = () => {
  let items = [
    "vigoda_1",
    "vigoda_11",
    "vigoda_21",
    "vigoda_31",
    "vigoda_41",
    "vigoda_51",
    "vigoda_61",
    "vigoda_71",
    "vigoda_81",
    "vigoda_91",
  ]
  return (
    <StarredResourceMemoryProvider initialValueForTesting={items}>
      <OverviewPane view={nResourceView(100)} />
    </StarredResourceMemoryProvider>
  )
}
