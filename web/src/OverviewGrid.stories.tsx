import React from "react"
import { MemoryRouter } from "react-router"
import OverviewGrid from "./OverviewGrid"
import { OverviewItem } from "./OverviewItemView"
import { nResourceView, tenResourceView, twoResourceView } from "./testdata"

type Resource = Proto.webviewResource

export default {
  title: "New UI/Overview/OverviewGrid",
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

function toItems(view: Proto.webviewView) {
  return (view.resources || []).map((res) => new OverviewItem(res))
}

export const TwoResources = () => (
  <OverviewGrid items={toItems(twoResourceView())} />
)

export const TenResources = () => {
  return <OverviewGrid items={toItems(tenResourceView())} />
}

export const OneHundredResources = () => {
  return <OverviewGrid items={toItems(nResourceView(100))} />
}
