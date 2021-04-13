import { mount, ReactWrapper } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import {
  ResourceGroupStatus,
  ResourceGroupStatusItem,
  ResourceGroupStatusSummaryItemCount,
  StatusCounts,
} from "./ResourceStatusSummary"
import Tooltip from "./Tooltip"

function expectStatusCounts(
  root: ReactWrapper,
  expected: { label: string; counts: number[] }[]
) {
  const itemRoot = root.find(ResourceGroupStatusItem)
  const actual = itemRoot.map((i) => {
    return {
      label: i.find(Tooltip).props().title,
      counts: i
        .find(ResourceGroupStatusSummaryItemCount)
        .map((e) => parseInt(e.text())),
    }
  })

  expect(actual).toEqual(expected)
}

it("shows the counts it's given", () => {
  const counts: StatusCounts = {
    total: 11,
    healthy: 0,
    warning: 2,
    unhealthy: 4,
    pending: 0,
  }
  const root = mount(
    <MemoryRouter>
      <ResourceGroupStatus
        counts={counts}
        healthyLabel="healthy"
        label="resources"
        unhealthyLabel="unhealthy"
        warningLabel="warning"
      />
    </MemoryRouter>
  )

  // "healthy" gets the denominator (total)
  // 0 counts are not rendered, except for "healthy"
  expectStatusCounts(root, [
    { label: "unhealthy", counts: [4] },
    { label: "warning", counts: [2] },
    { label: "healthy", counts: [0, 11] },
  ])
})
