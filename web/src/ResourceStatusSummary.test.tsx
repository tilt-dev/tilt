import { render, screen } from "@testing-library/react"
import React from "react"
import { MemoryRouter } from "react-router"
import { ResourceGroupStatus, StatusCounts } from "./ResourceStatusSummary"

function expectStatusCounts(expected: { label: string; counts: number[] }[]) {
  const actual = expected.map(({ label, counts }) => {
    const actualCounts: { label: string; counts: number[] } = {
      label,
      counts: [],
    }

    const actualCount =
      screen.queryByLabelText(`${label} count`)?.textContent ?? "0"
    actualCounts.counts.push(parseInt(actualCount))

    // Indicates an "out of count" for this label is expected
    if (counts[1]) {
      const actualCountOf =
        screen.queryByLabelText("Out of total resource count")?.textContent ??
        "0"
      actualCounts.counts.push(parseInt(actualCountOf))
    }

    return actualCounts
  })

  expect(actual).toEqual(expected)
}

const testCounts: StatusCounts = {
  totalEnabled: 11,
  healthy: 0,
  warning: 2,
  unhealthy: 4,
  pending: 0,
  disabled: 2,
}

it("shows the counts it's given", () => {
  render(
    <MemoryRouter>
      <ResourceGroupStatus
        counts={testCounts}
        healthyLabel="healthy"
        displayText="resources"
        labelText="Testing resource status summary"
        unhealthyLabel="unhealthy"
        warningLabel="warning"
        linkToLogFilters={true}
      />
    </MemoryRouter>
  )

  // "healthy" gets the denominator (totalEnabled)
  // 0 counts are not rendered, except for "healthy"
  expectStatusCounts([
    { label: "unhealthy", counts: [4] },
    { label: "warning", counts: [2] },
    { label: "healthy", counts: [0, 11] },
    { label: "disabled", counts: [2] },
  ])
})

it("links to warning and unhealthy resources when `linkToLogFilters` is true", () => {
  render(
    <MemoryRouter>
      <ResourceGroupStatus
        counts={testCounts}
        healthyLabel="healthy"
        displayText="resources"
        labelText="Testing resource status summary"
        unhealthyLabel="unhealthy"
        warningLabel="warning"
        linkToLogFilters={true}
      />
    </MemoryRouter>
  )

  expect(
    screen.getByRole("link", { name: "unhealthy count" })
  ).toBeInTheDocument()
  expect(
    screen.getByRole("link", { name: "warning count" })
  ).toBeInTheDocument()
})

it("does NOT link to warning and unhealthy resources when `linkToLogFilters` is false", () => {
  render(
    <MemoryRouter>
      <ResourceGroupStatus
        counts={testCounts}
        healthyLabel="healthy"
        displayText="resources"
        labelText="Testing resource status summary"
        unhealthyLabel="unhealthy"
        warningLabel="warning"
        linkToLogFilters={false}
      />
    </MemoryRouter>
  )

  expect(screen.queryByRole("link")).toBeNull()
})
