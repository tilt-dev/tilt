import { render, screen } from "@testing-library/react"
import React from "react"
import SidebarIcon from "./SidebarIcon"
import { ResourceStatus } from "./types"

const cases: Array<[string, ResourceStatus]> = [
  ["isPending", ResourceStatus.Pending],
  ["isHealthy", ResourceStatus.Healthy],
  ["isUnhealthy", ResourceStatus.Unhealthy],
  ["isBuilding", ResourceStatus.Building],
  ["isWarning", ResourceStatus.Warning],
  ["isNone", ResourceStatus.None],
]

describe("SidebarIcon", () => {
  test.each(cases)(
    "renders with the correct classes - %s",
    (className, status) => {
      render(
        <SidebarIcon status={status} alertCount={0} tooltipText={"help"} />
      )

      const iconWrapper = screen.getByTitle("help")
      expect(iconWrapper).toHaveClass(className)
    }
  )

  it("adds a tooltip with the correct text", () => {
    render(
      <SidebarIcon
        status={ResourceStatus.Unhealthy}
        alertCount={0}
        tooltipText="What a tip!"
      />
    )

    const tooltip = screen.getByRole("tooltip")
    expect(tooltip).toBeInTheDocument()
    expect(tooltip.title).toBe("What a tip!")
  })
})
