import React from "react"
import { MemoryRouter } from "react-router"
import OverviewTablePane from "./OverviewTablePane"
import { StarredResourceMemoryProvider } from "./StarredResourcesContext"
import { nResourceView, tenResourceView, twoResourceView } from "./testdata"

type UIResource = Proto.v1alpha1UIResource

export default {
  title: "New UI/OverviewTablePane",
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

export const TwoResources = () => <OverviewTablePane view={twoResourceView()} />

export const TenResources = () => <OverviewTablePane view={tenResourceView()} />

export const OneHundredResources = () => (
  <OverviewTablePane view={nResourceView(100)} />
)

export const OneHundredResourcesOneStar = () => (
  <StarredResourceMemoryProvider initialValueForTesting={["vigoda_2"]}>
    <OverviewTablePane view={nResourceView(100)} />
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
      <OverviewTablePane view={nResourceView(100)} />
    </StarredResourceMemoryProvider>
  )
}
