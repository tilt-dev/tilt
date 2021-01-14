import { fireEvent } from "@testing-library/dom"
import { mount } from "enzyme"
import React from "react"
import { act } from "react-dom/test-utils"
import { MemoryRouter } from "react-router-dom"
import MetricsDialog from "./MetricsDialog"
import { MenuButton } from "./OverviewResourceBar"
import { TwoResources } from "./OverviewResourceBar.stories"
import ShortcutsDialog from "./ShortcutsDialog"
import { SnapshotActionProvider } from "./snapshot"

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
      <SnapshotActionProvider value={snapshot}>
        {TwoResources()}
      </SnapshotActionProvider>
    </MemoryRouter>
  )

  expect(opened).toEqual(0)
  act(() => void fireEvent.keyDown(document.body, { key: "s" }))
  root.update()
  expect(opened).toEqual(1)
  root.unmount()
})

it("opens metrics dialog", () => {
  const root = mount(
    <MemoryRouter initialEntries={["/"]}>{TwoResources()}</MemoryRouter>
  )

  let buttons = root.find(MenuButton)
  expect(buttons).toHaveLength(5)

  let metricButton = buttons.at(2)
  expect(metricButton.getDOMNode().innerHTML).toEqual(
    expect.stringContaining("Metrics")
  )

  expect(root.find(MetricsDialog).props().open).toEqual(false)
  metricButton.simulate("click")
  expect(root.find(MetricsDialog).props().open).toEqual(true)
})
