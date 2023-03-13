import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React from "react"
import { MemoryRouter } from "react-router"
import StarredResourceBar from "./StarredResourceBar"
import { ResourceStatus } from "./types"

const TEST_RESOURCES = [
  { name: "foo", status: ResourceStatus.Healthy },
  { name: "bar", status: ResourceStatus.Unhealthy },
]

describe("StarredResourceBar", () => {
  let unstarSpy: jest.Mock

  beforeEach(() => {
    unstarSpy = jest.fn()
    render(
      <StarredResourceBar resources={TEST_RESOURCES} unstar={unstarSpy} />,
      { wrapper: MemoryRouter } as any
    )
  })

  it("renders the starred items", () => {
    expect(screen.getByRole("button", { name: "foo" })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "bar" })).toBeInTheDocument()
  })

  it("renders starred logs link", () => {
    expect(
      screen.getByRole("button", { name: "All Starred" })
    ).toBeInTheDocument()
  })

  it("calls unstar", () => {
    userEvent.click(screen.getByLabelText("Unstar foo"))

    expect(unstarSpy).toHaveBeenCalledWith("foo")
  })
})
