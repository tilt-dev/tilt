import { render, screen } from "@testing-library/react"
import React from "react"
import { MemoryRouter } from "react-router"
import {
  getDocumentTitle,
  ResourceGroupStatus,
  StatusCounts,
} from "./ResourceStatusSummary"

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

const testCountsUnhealthy: StatusCounts = {
  totalEnabled: 5,
  healthy: 3,
  unhealthy: 2,
  disabled: 0,
  pending: 0,
  warning: 0,
}

const testCountsHealthy: StatusCounts = {
  totalEnabled: 5,
  healthy: 5,
  disabled: 3,
  unhealthy: 0,
  pending: 0,
  warning: 0,
}

const testCountsPending: StatusCounts = {
  totalEnabled: 5,
  healthy: 3,
  pending: 2,
  disabled: 2,
  unhealthy: 0,
  warning: 0,
}

const testCountsAllDisabled: StatusCounts = {
  totalEnabled: 0,
  disabled: 5,
  healthy: 0,
  pending: 0,
  unhealthy: 0,
  warning: 0,
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

describe("document metadata based on resource statuses", () => {
  test.each<
    [
      caseName: string,
      args: {
        counts: StatusCounts
        isSnapshot: boolean
        isSocketConnected: boolean
      },
      title: string,
      faviconColor: string
    ]
  >([
    [
      "no socket connection",
      {
        counts: testCountsUnhealthy,
        isSnapshot: false,
        isSocketConnected: false,
      },
      "Disconnected ┊ Tilt",
      "gray",
    ],
    [
      "viewing a snapshot",
      {
        counts: testCountsUnhealthy,
        isSnapshot: true,
        isSocketConnected: false,
      },
      "Snapshot: ✖︎ 2 ┊ Tilt",
      "red",
    ],
    [
      "some resources are unhealthy",
      {
        counts: testCountsUnhealthy,
        isSnapshot: false,
        isSocketConnected: true,
      },
      "✖︎ 2 ┊ Tilt",
      "red",
    ],
    [
      "all enabled resources are healthy",
      { counts: testCountsHealthy, isSnapshot: false, isSocketConnected: true },
      "✔︎ 5/5 ┊ Tilt",
      "green",
    ],
    [
      "some resources are pending",
      { counts: testCountsPending, isSnapshot: false, isSocketConnected: true },
      "… 3/5 ┊ Tilt",
      "gray",
    ],
    [
      "all resources are disabled",
      {
        counts: testCountsAllDisabled,
        isSnapshot: false,
        isSocketConnected: true,
      },
      "✔︎ 0/0 ┊ Tilt",
      "gray",
    ],
  ])(
    "has correct title and icon when %p",
    (_testTitle, args, expectedTitle, expectedIconColor) => {
      const { counts, isSnapshot, isSocketConnected } = args
      const { title, faviconHref } = getDocumentTitle(
        counts,
        isSnapshot,
        isSocketConnected
      )
      expect(title).toBe(expectedTitle)
      expect(faviconHref).toStrictEqual(
        expect.stringContaining(expectedIconColor)
      )
    }
  )
})
