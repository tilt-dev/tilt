import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import fetchMock, { MockResponseObject } from "fetch-mock"
import React from "react"
import AnalyticsNudge, { NUDGE_TIMEOUT_MS } from "./AnalyticsNudge"
import {
  cleanupMockAnalyticsCalls,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"

function mockAnalyticsOptInOnce(optIn = true, error = false) {
  const response: MockResponseObject = error ? { status: 500 } : {}
  const opt = optIn ? "opt-in" : "opt-out"
  fetchMock.postOnce("//localhost/api/analytics_opt", response, {
    body: { opt },
  })
}

describe("AnalyticsNudge", () => {
  beforeEach(() => mockAnalyticsCalls())

  afterEach(() => cleanupMockAnalyticsCalls())

  it("shows nudge if `needNudge` is true and no request has been made", () => {
    render(<AnalyticsNudge needsNudge={true} />)

    expect(screen.getByLabelText("Tilt analytics options")).toBeInTheDocument()
  })

  it("does NOT show nudge if `needsNudge` is false and no request has been made", () => {
    render(<AnalyticsNudge needsNudge={false} />)

    expect(screen.queryByLabelText("Tilt analytics options")).toBeNull()
  })

  it("does NOT show nudge after it has been dismissed", async () => {
    mockAnalyticsOptInOnce(false)

    render(<AnalyticsNudge needsNudge={true} />)

    expect(screen.getByLabelText("Tilt analytics options")).toBeInTheDocument()

    userEvent.click(screen.getByRole("button", { name: /nope/i }))

    await waitFor(() => {
      userEvent.click(screen.getByRole("button", { name: /dismiss/i }))
    })

    expect(screen.queryByLabelText("Tilt analytics options")).toBeNull()
  })

  it("shows request-in-progress message when a request is in progress", () => {
    mockAnalyticsOptInOnce(false)

    render(<AnalyticsNudge needsNudge={true} />)

    userEvent.click(screen.getByRole("button", { name: /nope/i }))

    expect(screen.getByTestId("opt-loading"))
  })

  it("shows success message for opt out", async () => {
    mockAnalyticsOptInOnce(false)
    render(<AnalyticsNudge needsNudge={true} />)

    userEvent.click(screen.getByRole("button", { name: /nope/i }))

    await waitFor(() => {
      expect(screen.getByTestId("optout-success")).toBeInTheDocument()
    })
  })

  it("shows success message for opt in", async () => {
    mockAnalyticsOptInOnce(true)
    render(<AnalyticsNudge needsNudge={true} />)

    userEvent.click(screen.getByRole("button", { name: /I'm in/i }))

    await waitFor(() => {
      expect(screen.getByTestId("option-success")).toBeInTheDocument()
    })
  })

  it("shows a failure message when request fails", async () => {
    mockAnalyticsOptInOnce(true, true)
    render(<AnalyticsNudge needsNudge={true} />)

    userEvent.click(screen.getByRole("button", { name: /I'm in/i }))

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument()
    })
  })

  it("dismisses the success message after a set delay", async () => {
    jest.useFakeTimers()
    mockAnalyticsOptInOnce(true)
    render(<AnalyticsNudge needsNudge={true} />)

    userEvent.click(screen.getByRole("button", { name: /I'm in/i }))

    await waitFor(() => {
      expect(screen.getByTestId("option-success")).toBeInTheDocument()
    })

    jest.advanceTimersByTime(NUDGE_TIMEOUT_MS)

    expect(screen.queryByLabelText("Tilt analytics options")).toBeNull()
  })
})
