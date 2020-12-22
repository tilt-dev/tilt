import { fireEvent } from "@testing-library/dom"
import { mount } from "enzyme"
import React from "react"
import { act } from "react-dom/test-utils"
import { MemoryRouter } from "react-router-dom"
import { TwoResources } from "./OverviewResourceBar.stories"
import ShortcutsDialog from "./ShortcutsDialog"

it("renders shortcuts dialog on ?", () => {
  const root = mount(
    <MemoryRouter initialEntries={["/"]}>{TwoResources()}</MemoryRouter>
  )

  expect(root.find(ShortcutsDialog).props().open).toEqual(false)
  act(() => void fireEvent.keyDown(document.body, { key: "?" }))
  root.update()
  expect(root.find(ShortcutsDialog).props().open).toEqual(true)
  root.unmount()
})
