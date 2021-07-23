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

const testCounts: StatusCounts = {
  total: 11,
  healthy: 0,
  warning: 2,
  unhealthy: 4,
  pending: 0,
}

it("shows the counts it's given", () => {
  const root = mount(
    <MemoryRouter>
      <ResourceGroupStatus
        counts={testCounts}
        healthyLabel="healthy"
        label="resources"
        unhealthyLabel="unhealthy"
        warningLabel="warning"
        linkToLogFilters={true}
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

it("links to warning and unhealthy resources when `linkToLogFilters` is true", () => {
  const root = mount(
    <MemoryRouter>
      <ResourceGroupStatus
        counts={testCounts}
        healthyLabel="healthy"
        label="resources"
        unhealthyLabel="unhealthy"
        warningLabel="warning"
        linkToLogFilters={true}
      />
    </MemoryRouter>
  )

  const warningLinkCount = root
    .find(ResourceGroupStatusItem)
    .filterWhere((item) => item.props().label === "warning")
    .find("a").length
  const errorLinkCount = root
    .find(ResourceGroupStatusItem)
    .filterWhere((item) => item.props().label === "unhealthy")
    .find("a").length

  expect(warningLinkCount).toBe(1)
  expect(errorLinkCount).toBe(1)
})

it("does NOT link to warning and unhealthy resources when `linkToLogFilters` is false", () => {
  const root = mount(
    <MemoryRouter>
      <ResourceGroupStatus
        counts={testCounts}
        healthyLabel="healthy"
        label="resources"
        unhealthyLabel="unhealthy"
        warningLabel="warning"
        linkToLogFilters={false}
      />
    </MemoryRouter>
  )

  const linkCount = root.find(ResourceGroupStatusItem).find("a").length

  expect(linkCount).toBe(0)
})
