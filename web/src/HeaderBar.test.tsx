import { fireEvent } from "@testing-library/dom"
import { mount } from "enzyme"
import React from "react"
import { act } from "react-dom/test-utils"
import { MemoryRouter } from "react-router-dom"
import { TwoResources } from "./HeaderBar.stories"
import HelpDialog from "./HelpDialog"
import { SnapshotActionValueProvider } from "./snapshot"

it("renders shortcuts dialog on ?", () => {
  const root = mount(
    <MemoryRouter initialEntries={["/"]}>{TwoResources()}</MemoryRouter>
  )

  expect(root.find(HelpDialog).props().open).toEqual(false)
  act(() => void fireEvent.keyDown(document.body, { key: "?" }))
  root.update()
  expect(root.find(HelpDialog).props().open).toEqual(true)
  root.unmount()
})

it("opens snapshot modal on s", () => {
  let opened = 0
  let snapshot = {
    enabled: true,
    openModal: () => {
      opened++
    },
  }
  const root = mount(
    <MemoryRouter initialEntries={["/"]}>
      <SnapshotActionValueProvider value={snapshot}>
        {TwoResources()}
      </SnapshotActionValueProvider>
    </MemoryRouter>
  )

  expect(opened).toEqual(0)
  act(() => void fireEvent.keyDown(document.body, { key: "s" }))
  root.update()
  expect(opened).toEqual(1)
  root.unmount()
})
