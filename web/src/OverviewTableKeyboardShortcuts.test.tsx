import { render } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React from "react"
import { RowValues } from "./OverviewTableColumns"
import { OverviewTableKeyboardShortcuts } from "./OverviewTableKeyboardShortcuts"
import { ResourceSelectionProvider } from "./ResourceSelectionContext"

const rows = ["a", "b", "c"].map((name) => ({ name } as RowValues))

function renderShortcuts(focused: string) {
  const setFocused = jest.fn()
  render(
    <ResourceSelectionProvider>
      <OverviewTableKeyboardShortcuts
        rows={rows}
        focused={focused}
        setFocused={setFocused}
      />
    </ResourceSelectionProvider>
  )
  return setFocused
}

describe("OverviewTableKeyboardShortcuts", () => {
  it("moves down on 'j'", () => {
    const setFocused = renderShortcuts("b")
    userEvent.keyboard("j")
    expect(setFocused).toHaveBeenCalledWith("c")
  })

  it("moves up on 'k'", () => {
    const setFocused = renderShortcuts("b")
    userEvent.keyboard("k")
    expect(setFocused).toHaveBeenCalledWith("a")
  })

  it("moves down on the down arrow", () => {
    const setFocused = renderShortcuts("b")
    userEvent.keyboard("{ArrowDown}")
    expect(setFocused).toHaveBeenCalledWith("c")
  })

  it("moves up on the up arrow", () => {
    const setFocused = renderShortcuts("b")
    userEvent.keyboard("{ArrowUp}")
    expect(setFocused).toHaveBeenCalledWith("a")
  })

  it("starts at the top when nothing is focused", () => {
    const setFocused = renderShortcuts("")
    userEvent.keyboard("{ArrowDown}")
    expect(setFocused).toHaveBeenCalledWith("a")
  })
})
