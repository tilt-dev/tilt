import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React from "react"
import { TiltInfoTooltip } from "./Tooltip"

describe("TiltInfoTooltip", () => {
  beforeEach(() => {
    localStorage.clear()
  })

  afterEach(() => {
    localStorage.clear()
  })

  it("hides info button when clicked", () => {
    let { container } = render(
      <TiltInfoTooltip title="Hello!" dismissId="test-tooltip" open={true} />
    )

    expect(container.querySelectorAll("svg").length).toEqual(1)
    userEvent.hover(container.querySelector("svg")!)

    expect(screen.getByText("Don't show this tip")).toBeInTheDocument()
    userEvent.click(screen.getByText("Don't show this tip"))
    expect(screen.queryByText("Don't show this tip")).not.toBeInTheDocument()

    // and the setting is in localStorage
    expect(localStorage.getItem("tooltip-dismissed-test-tooltip")).toEqual(
      "true"
    )
  })
})
