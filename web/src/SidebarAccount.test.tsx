import { fireEvent } from "@testing-library/dom"
import { mount } from "enzyme"
import React from "react"
import { act } from "react-dom/test-utils"
import ReactModal from "react-modal"
import { MemoryRouter } from "react-router-dom"
import ShortcutsDialog from "./ShortcutsDialog"
import SidebarAccount, { SidebarAccountRoot } from "./SidebarAccount"

beforeEach(() => {
  ReactModal.setAppElement(document.body)
})

it("renders nothing on a Tilt Snapshot", () => {
  const root = mount(
    <MemoryRouter initialEntries={["/"]}>
      <SidebarAccount
        tiltCloudUsername=""
        tiltCloudSchemeHost="http://cloud.tilt.dev"
        tiltCloudTeamID=""
        tiltCloudTeamName=""
        isSnapshot={true}
      />
    </MemoryRouter>
  )

  expect(root.find(SidebarAccountRoot)).toHaveLength(0)
  root.unmount()
})

it("renders shortcuts dialog on ?", () => {
  const root = mount(
    <MemoryRouter initialEntries={["/"]}>
      <SidebarAccount
        tiltCloudUsername="amaia"
        tiltCloudSchemeHost="http://cloud.tilt.dev"
        tiltCloudTeamID="cactus inc"
        tiltCloudTeamName=""
        isSnapshot={false}
      />
    </MemoryRouter>
  )

  expect(root.find(ShortcutsDialog).props().open).toEqual(false)
  act(() => void fireEvent.keyDown(document.body, { key: "?" }))
  root.update()
  expect(root.find(ShortcutsDialog).props().open).toEqual(true)
  root.unmount()
})
